//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/firewall"
	keepnet "github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/operator"
)

// This file closes the key-CUSTODY gap that the dealer-DKG shape-(B) harness
// (roast_shapeb_libp2p_multiproc_e2e_frost_native_test.go) left open. There, DKG was the
// centralized dev "dealer" call (frost_tbtc_run_dkg) run once and the encrypted key group
// COPIED into every worker, so each worker physically held the whole key group. That is
// the transitional dealer path, which the engine HARD-DISABLES in production
// (enforce_bootstrap_dealer_dkg_disabled_in_production: "production requires distributed
// DKG wiring").
//
// Here every worker runs the REAL distributed FROST DKG (frost_tbtc_dkg_part1/2/3) over
// real libp2p and ends up holding ONLY ITS OWN key package - no node ever sees the whole
// key group. Then the n nodes threshold-sign the message with the low-level engine path
// (GenerateNoncesAndCommitments/NewSigningPackage/Sign/Aggregate), each aggregating its
// own BIP-340 signature independently. So this is production-shaped end to end: distributed
// keygen + threshold signing, n separate OS processes, real transport, true per-node share
// custody.
//
// ROUND-2 CONFIDENTIALITY (why the encryption below is mandatory, not decorative): FROST
// DKG round-2 packages are per-recipient SECRET shares - package i->j is f_i(j). The bus is
// broadcast-only, and if round-2 packages traveled in clear, any node could collect f_i(j)
// for all i and RECONSTRUCT j's secret share (sum_i f_i(j)), defeating the threshold
// property entirely. So each round-2 package is sealed to its recipient with secp256k1
// ECDH(sender_op_key, recipient_op_key) + AES-256-GCM before it is broadcast; only the
// intended recipient can open it. (Round-1 packages are public commitments and are
// broadcast in clear, as the protocol intends.)
//
// MECHANISM: same subprocess-helper pattern as shape-(B) - the orchestrator re-execs THIS
// test binary (already linked against libfrost_tbtc) as n workers. There is NO central DKG
// and NO shared state file: the low-level FROST path is stateless, so a worker needs only
// the development profile env, not a persisted signer state.

const (
	ddkgWorkerEnv  = "FROST_DDKG_WORKER"
	ddkgConfigEnv  = "FROST_DDKG_CONFIG"
	ddkgTopic      = "frost-distributed-dkg-and-sign"
	ddkgSigPrefix  = "DDKG_SIGNATURE="
	ddkgErrPrefix  = "DDKG_ERROR="
	ddkgSkipPrefix = "DDKG_SKIP="
	ddkgVKeyPrefix = "DDKG_GROUPKEY=" // diagnostic only

	phaseRound1   = "dkg-r1"
	phaseRound2   = "dkg-r2"
	phaseGroupKey = "dkg-groupkey"
	phaseCommit   = "sign-commit"
	phaseShare    = "sign-share"
)

type ddkgMember struct {
	Index        int    `json:"index"`
	OperatorDHex string `json:"operator_d_hex"`
	Port         int    `json:"port"`
	Multiaddr    string `json:"multiaddr"`
}

type ddkgConfig struct {
	N          int          `json:"n"`
	Threshold  int          `json:"threshold"`
	MessageHex string       `json:"message_hex"`
	Topic      string       `json:"topic"`
	Members    []ddkgMember `json:"members"`
}

// ddkgMsg is the single wire type for every phase. Recipient==0 means broadcast-to-all;
// Recipient==j means the payload is a per-recipient bundle (round-2). It implements both
// net.TaggedMarshaler and net.TaggedUnmarshaler.
type ddkgMsg struct {
	Phase     string `json:"phase"`
	Sender    int    `json:"sender"`
	Recipient int    `json:"recipient"`
	Payload   []byte `json:"payload"`
}

func (m *ddkgMsg) Type() string             { return "frost/distributed-dkg-sign/v1" }
func (m *ddkgMsg) Marshal() ([]byte, error) { return json.Marshal(m) }
func (m *ddkgMsg) Unmarshal(b []byte) error { return json.Unmarshal(b, m) }

func TestRealCgoInteractiveSigning_Libp2pMultiProc_DistributedDKG(t *testing.T) {
	if idxStr := os.Getenv(ddkgWorkerEnv); idxStr != "" {
		runDdkgWorker(t, idxStr)
		return
	}
	runDdkgOrchestrator(t, 3, 2)
}

func runDdkgOrchestrator(t *testing.T, n int, threshold uint16) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// One transport/ECDH operator key per member + a free port + its libp2p peer id, so
	// the whole peer table (every worker's bootstrap list) is known before launch.
	derivation, err := libp2p.Connect(
		ctx,
		libp2p.Config{Port: freeTCPPort(t)},
		mustGenOperatorKey(t),
		firewall.Disabled,
		idleRetransmissionTicker(),
	)
	if err != nil {
		t.Fatalf("derivation provider: %v", err)
	}

	members := make([]ddkgMember, 0, n)
	for i := 0; i < n; i++ {
		priv, pub, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (member %d): %v", i+1, err)
		}
		peerID, err := derivation.CreateTransportIdentifier(pub)
		if err != nil {
			t.Fatalf("peer id (member %d): %v", i+1, err)
		}
		port := freeTCPPort(t)
		members = append(members, ddkgMember{
			Index:        i + 1,
			OperatorDHex: hex.EncodeToString(priv.D.Bytes()),
			Port:         port,
			Multiaddr:    fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, peerID),
		})
	}

	message := make([]byte, 32)
	for i := range message {
		message[i] = 0x42
	}
	cfg := ddkgConfig{
		N:          n,
		Threshold:  int(threshold),
		MessageHex: hex.EncodeToString(message),
		Topic:      ddkgTopic,
		Members:    members,
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configPath := t.TempDir() + "/ddkg-config.json"
	if err := os.WriteFile(configPath, cfgBytes, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	type result struct {
		index  int
		output string
		err    error
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	for i := range members {
		wg.Add(1)
		go func(idx int, m ddkgMember) {
			defer wg.Done()
			cmd := exec.CommandContext(ctx, os.Args[0],
				"-test.run", "^TestRealCgoInteractiveSigning_Libp2pMultiProc_DistributedDKG$",
				"-test.v", "-test.timeout=170s",
			)
			cmd.Env = withEnvOverrides(os.Environ(), map[string]string{
				ddkgWorkerEnv:                         strconv.Itoa(m.Index),
				ddkgConfigEnv:                         configPath,
				"TBTC_SIGNER_PROFILE":                 "development",
				"TBTC_SIGNER_ENFORCE_PROVENANCE_GATE": "false",
				frostSubprocessSkipPrefixEnv:          ddkgSkipPrefix,
			})
			out, err := cmd.CombinedOutput()
			results[idx] = result{index: m.Index, output: string(out), err: err}
		}(i, members[i])
	}
	wg.Wait()

	var winning string
	winners := 0
	for _, r := range results {
		if skip := extractPrefixed(r.output, ddkgSkipPrefix); skip != "" {
			t.Skipf("member %d skipped: %s", r.index, skip)
		}
		sig := extractPrefixed(r.output, ddkgSigPrefix)
		if sig == "" {
			t.Fatalf("member %d produced no signature (err=%v):\n%s", r.index, r.err, indentTail(r.output, 40))
		}
		raw, err := hex.DecodeString(sig)
		if err != nil || len(raw) != 64 {
			t.Fatalf("member %d emitted a bad signature %q (decErr=%v len=%d)", r.index, sig, err, len(raw))
		}
		if winning == "" {
			winning = sig
		} else if sig != winning {
			t.Fatalf("member %d produced a different signature than a peer:\n  got  %s\n  want %s", r.index, sig, winning)
		}
		winners++
	}
	if winners != n {
		t.Fatalf("expected all %d distributed-DKG nodes to aggregate the signature, got %d", n, winners)
	}
	t.Logf("distributed-DKG: %d separate-process nodes ran real FROST part1/2/3 over libp2p (each holding ONLY its own share) and threshold-signed to the same BIP-340 signature %s…", n, winning[:16])
}

func runDdkgWorker(t *testing.T, idxStr string) {
	index, err := strconv.Atoi(idxStr)
	if err != nil {
		fmt.Printf("%sbad worker index %q: %v\n", ddkgErrPrefix, idxStr, err)
		return
	}
	cfgBytes, err := os.ReadFile(os.Getenv(ddkgConfigEnv))
	if err != nil {
		fmt.Printf("%sread config: %v\n", ddkgErrPrefix, err)
		return
	}
	var cfg ddkgConfig
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		fmt.Printf("%sparse config: %v\n", ddkgErrPrefix, err)
		return
	}

	var self ddkgMember
	peers := make([]string, 0, cfg.N-1)
	others := make([]int, 0, cfg.N-1)
	pubByIndex := map[int]*operator.PublicKey{}
	for _, m := range cfg.Members {
		key, err := operatorKeyFromDHex(m.OperatorDHex)
		if err != nil {
			fmt.Printf("%sreconstruct member %d key: %v\n", ddkgErrPrefix, m.Index, err)
			return
		}
		pubByIndex[m.Index] = &key.PublicKey
		if m.Index == index {
			self = m
		} else {
			peers = append(peers, m.Multiaddr)
			others = append(others, m.Index)
		}
	}
	if self.Index == 0 {
		fmt.Printf("%smember %d not in config\n", ddkgErrPrefix, index)
		return
	}
	selfKey, err := operatorKeyFromDHex(self.OperatorDHex)
	if err != nil {
		fmt.Printf("%sreconstruct self key: %v\n", ddkgErrPrefix, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 160*time.Second)
	defer cancel()

	provider, err := libp2p.Connect(
		ctx,
		libp2p.Config{Port: self.Port, Peers: peers, Bootstrap: true},
		selfKey,
		firewall.Disabled,
		periodicRetransmissionTicker(ctx, 300*time.Millisecond),
	)
	if err != nil {
		fmt.Printf("%slibp2p connect: %v\n", ddkgErrPrefix, err)
		return
	}
	if err := waitForPeers(ctx, provider, cfg.N-1, 60*time.Second); err != nil {
		fmt.Printf("%swait for peers: %v\n", ddkgErrPrefix, err)
		return
	}
	channel, err := provider.BroadcastChannelFor(cfg.Topic)
	if err != nil {
		fmt.Printf("%sbroadcast channel: %v\n", ddkgErrPrefix, err)
		return
	}
	channel.SetUnmarshaler(func() keepnet.TaggedUnmarshaler { return &ddkgMsg{} })
	// Accept only the known member operator keys (membership = authentication here).
	known := map[string]bool{}
	for _, pub := range pubByIndex {
		known[string(operator.MarshalCompressed(pub))] = true
	}
	if err := channel.SetFilter(func(pub *operator.PublicKey) bool {
		return known[string(operator.MarshalCompressed(pub))]
	}); err != nil {
		fmt.Printf("%sset filter: %v\n", ddkgErrPrefix, err)
		return
	}

	collector := newDdkgCollector(index)
	channel.Recv(ctx, func(m keepnet.Message) {
		msg, ok := m.Payload().(*ddkgMsg)
		if !ok || msg.Sender == index {
			return
		}
		collector.put(msg.Phase, msg.Sender, msg.Payload)
	})

	// Gossipsub mesh warmup before the first round (the periodic ticker also resends).
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		fmt.Printf("%scontext done during warmup\n", ddkgErrPrefix)
		return
	}

	engine := &buildTaggedTBTCSignerEngine{}
	selfID := buildTaggedTBTCSignerTestIdentifier(byte(index))

	// ---- DKG part 1: broadcast my public round-1 commitment package ----
	part1, err := engine.Part1(selfID, uint16(cfg.N), uint16(cfg.Threshold))
	if err != nil {
		if reportFrostSubprocessSkip("distributed DKG part1", err) {
			return
		}
		fmt.Printf("%spart1: %v\n", ddkgErrPrefix, err)
		return
	}
	if err := ddkgSend(ctx, channel, phaseRound1, index, 0, mustJSON(part1.Package)); err != nil {
		fmt.Printf("%ssend round1: %v\n", ddkgErrPrefix, err)
		return
	}
	r1Raw, err := collector.collect(ctx, phaseRound1, others, 45*time.Second)
	if err != nil {
		fmt.Printf("%scollect round1: %v\n", ddkgErrPrefix, err)
		return
	}
	round1Packages := make([]*NativeFROSTDKGRound1Package, 0, len(others))
	for _, idx := range others {
		var pkg NativeFROSTDKGRound1Package
		if err := json.Unmarshal(r1Raw[idx], &pkg); err != nil {
			fmt.Printf("%sdecode round1 from %d: %v\n", ddkgErrPrefix, idx, err)
			return
		}
		round1Packages = append(round1Packages, &pkg)
	}

	// ---- DKG part 2: produce per-recipient secret packages, SEAL each to its recipient ----
	part2, err := engine.Part2(part1.SecretPackage, round1Packages)
	if err != nil {
		if reportFrostSubprocessSkip("distributed DKG part2", err) {
			return
		}
		fmt.Printf("%spart2: %v\n", ddkgErrPrefix, err)
		return
	}
	sealedBundle := map[int][]byte{} // recipient index -> ciphertext(round2 package)
	for _, recipient := range others {
		recipientID := buildTaggedTBTCSignerTestIdentifier(byte(recipient))
		var pkgForRecipient *NativeFROSTDKGRound2Package
		for _, pkg := range part2.Packages {
			if pkg.Identifier == recipientID {
				pkgForRecipient = pkg
				break
			}
		}
		if pkgForRecipient == nil {
			fmt.Printf("%smissing round2 package for recipient %d\n", ddkgErrPrefix, recipient)
			return
		}
		sealed, err := sealForPeer(selfKey.D.Bytes(), pubByIndex[recipient], mustJSON(pkgForRecipient))
		if err != nil {
			fmt.Printf("%sseal round2 for %d: %v\n", ddkgErrPrefix, recipient, err)
			return
		}
		sealedBundle[recipient] = sealed
	}
	if err := ddkgSend(ctx, channel, phaseRound2, index, 0, mustJSON(sealedBundle)); err != nil {
		fmt.Printf("%ssend round2: %v\n", ddkgErrPrefix, err)
		return
	}
	r2Raw, err := collector.collect(ctx, phaseRound2, others, 45*time.Second)
	if err != nil {
		fmt.Printf("%scollect round2: %v\n", ddkgErrPrefix, err)
		return
	}
	round2Packages := make([]*NativeFROSTDKGRound2Package, 0, len(others))
	for _, senderIdx := range others {
		var bundle map[int][]byte
		if err := json.Unmarshal(r2Raw[senderIdx], &bundle); err != nil {
			fmt.Printf("%sdecode round2 bundle from %d: %v\n", ddkgErrPrefix, senderIdx, err)
			return
		}
		sealed, ok := bundle[index]
		if !ok {
			fmt.Printf("%sround2 bundle from %d has nothing for me\n", ddkgErrPrefix, senderIdx)
			return
		}
		plain, err := openFromPeer(selfKey.D.Bytes(), pubByIndex[senderIdx], sealed)
		if err != nil {
			fmt.Printf("%sopen round2 from %d: %v\n", ddkgErrPrefix, senderIdx, err)
			return
		}
		var pkg NativeFROSTDKGRound2Package
		if err := json.Unmarshal(plain, &pkg); err != nil {
			fmt.Printf("%sdecode round2 from %d: %v\n", ddkgErrPrefix, senderIdx, err)
			return
		}
		pkg.SenderIdentifier = buildTaggedTBTCSignerTestIdentifier(byte(senderIdx))
		round2Packages = append(round2Packages, &pkg)
	}

	// ---- DKG part 3: derive MY key package + the shared group public key ----
	dkgResult, err := engine.Part3(part2.SecretPackage, round1Packages, round2Packages)
	if err != nil {
		if reportFrostSubprocessSkip("distributed DKG part3", err) {
			return
		}
		fmt.Printf("%spart3: %v\n", ddkgErrPrefix, err)
		return
	}
	myKeyPackage := dkgResult.KeyPackage
	groupPublicKey := dkgResult.PublicKeyPackage

	// ---- agreement check: every node must derive the same group verifying key ----
	if err := ddkgSend(ctx, channel, phaseGroupKey, index, 0, []byte(groupPublicKey.VerifyingKey)); err != nil {
		fmt.Printf("%ssend groupkey: %v\n", ddkgErrPrefix, err)
		return
	}
	gkRaw, err := collector.collect(ctx, phaseGroupKey, others, 30*time.Second)
	if err != nil {
		fmt.Printf("%scollect groupkey: %v\n", ddkgErrPrefix, err)
		return
	}
	for _, idx := range others {
		if string(gkRaw[idx]) != groupPublicKey.VerifyingKey {
			fmt.Printf("%sgroup key disagreement with member %d\n", ddkgErrPrefix, idx)
			return
		}
	}

	// ---- threshold sign (low-level path): commit -> share -> aggregate, all over libp2p ----
	nonces, commitmentID, commitmentData, err := engine.GenerateNoncesAndCommitments(
		myKeyPackage.Identifier, myKeyPackage.Data,
	)
	if err != nil {
		if reportFrostSubprocessSkip("generate nonces and commitments", err) {
			return
		}
		fmt.Printf("%sgenerate nonces: %v\n", ddkgErrPrefix, err)
		return
	}
	if err := ddkgSend(ctx, channel, phaseCommit, index, 0,
		mustJSON(nativeFROSTCommitment{Identifier: commitmentID, Data: commitmentData})); err != nil {
		fmt.Printf("%ssend commitment: %v\n", ddkgErrPrefix, err)
		return
	}
	commitRaw, err := collector.collect(ctx, phaseCommit, others, 45*time.Second)
	if err != nil {
		fmt.Printf("%scollect commitments: %v\n", ddkgErrPrefix, err)
		return
	}
	commitments := []nativeFROSTCommitment{{Identifier: commitmentID, Data: commitmentData}}
	for _, idx := range others {
		var c nativeFROSTCommitment
		if err := json.Unmarshal(commitRaw[idx], &c); err != nil {
			fmt.Printf("%sdecode commitment from %d: %v\n", ddkgErrPrefix, idx, err)
			return
		}
		commitments = append(commitments, c)
	}
	// Deterministic order across nodes so every node builds the SAME signing package.
	sort.Slice(commitments, func(i, j int) bool { return commitments[i].Identifier < commitments[j].Identifier })

	message, err := hex.DecodeString(cfg.MessageHex)
	if err != nil {
		fmt.Printf("%sbad message: %v\n", ddkgErrPrefix, err)
		return
	}
	signingPackage, err := engine.NewSigningPackage(message, commitments)
	if err != nil {
		if reportFrostSubprocessSkip("new signing package", err) {
			return
		}
		fmt.Printf("%snew signing package: %v\n", ddkgErrPrefix, err)
		return
	}
	shareID, shareData, err := engine.Sign(signingPackage, nonces, myKeyPackage.Identifier, myKeyPackage.Data)
	if err != nil {
		if reportFrostSubprocessSkip("sign", err) {
			return
		}
		fmt.Printf("%ssign: %v\n", ddkgErrPrefix, err)
		return
	}
	if err := ddkgSend(ctx, channel, phaseShare, index, 0,
		mustJSON(nativeFROSTSignatureShare{Identifier: shareID, Data: shareData})); err != nil {
		fmt.Printf("%ssend share: %v\n", ddkgErrPrefix, err)
		return
	}
	shareRaw, err := collector.collect(ctx, phaseShare, others, 45*time.Second)
	if err != nil {
		fmt.Printf("%scollect shares: %v\n", ddkgErrPrefix, err)
		return
	}
	shares := []nativeFROSTSignatureShare{{Identifier: shareID, Data: shareData}}
	for _, idx := range others {
		var s nativeFROSTSignatureShare
		if err := json.Unmarshal(shareRaw[idx], &s); err != nil {
			fmt.Printf("%sdecode share from %d: %v\n", ddkgErrPrefix, idx, err)
			return
		}
		shares = append(shares, s)
	}
	sort.Slice(shares, func(i, j int) bool { return shares[i].Identifier < shares[j].Identifier })

	signature, err := engine.Aggregate(signingPackage, shares, groupPublicKey)
	if err != nil {
		if reportFrostSubprocessSkip("aggregate", err) {
			return
		}
		fmt.Printf("%saggregate: %v\n", ddkgErrPrefix, err)
		return
	}
	if len(signature) != 64 {
		fmt.Printf("%sunexpected signature length %d\n", ddkgErrPrefix, len(signature))
		return
	}
	fmt.Printf("%s%s\n", ddkgSigPrefix, hex.EncodeToString(signature))
}

// ---- collector ----

type ddkgCollector struct {
	mu      sync.Mutex
	self    int
	byPhase map[string]map[int][]byte
	bump    chan struct{}
}

func newDdkgCollector(self int) *ddkgCollector {
	return &ddkgCollector{self: self, byPhase: map[string]map[int][]byte{}, bump: make(chan struct{}, 1024)}
}

func (c *ddkgCollector) put(phase string, sender int, payload []byte) {
	c.mu.Lock()
	m := c.byPhase[phase]
	if m == nil {
		m = map[int][]byte{}
		c.byPhase[phase] = m
	}
	if _, ok := m[sender]; !ok {
		m[sender] = payload
	}
	c.mu.Unlock()
	select {
	case c.bump <- struct{}{}:
	default:
	}
}

func (c *ddkgCollector) collect(ctx context.Context, phase string, want []int, timeout time.Duration) (map[int][]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		c.mu.Lock()
		have := map[int][]byte{}
		if m := c.byPhase[phase]; m != nil {
			for _, s := range want {
				if p, ok := m[s]; ok {
					have[s] = p
				}
			}
		}
		c.mu.Unlock()
		if len(have) == len(want) {
			return have, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("phase %s: got %d of %d before timeout", phase, len(have), len(want))
		}
		select {
		case <-c.bump:
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ---- helpers ----

func ddkgSend(ctx context.Context, channel keepnet.BroadcastChannel, phase string, sender, recipient int, payload []byte) error {
	return channel.Send(ctx, &ddkgMsg{Phase: phase, Sender: sender, Recipient: recipient, Payload: payload})
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("ddkg marshal: %v", err))
	}
	return b
}

// ecdhKey derives a shared AES key from secp256k1 ECDH between my scalar and a peer's
// operator public key. Symmetric: ecdhKey(myD, peerPub) == ecdhKey(peerD, myPub).
func ecdhKey(myD []byte, peerPub *operator.PublicKey) []byte {
	sx, _ := local_v1.DefaultCurve.ScalarMult(peerPub.X, peerPub.Y, myD)
	sum := sha256.Sum256(sx.Bytes())
	return sum[:]
}

func sealForPeer(myD []byte, peerPub *operator.PublicKey, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(ecdhKey(myD, peerPub))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func openFromPeer(myD []byte, peerPub *operator.PublicKey, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(ecdhKey(myD, peerPub))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, fmt.Errorf("sealed payload too short")
	}
	nonce, ct := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
