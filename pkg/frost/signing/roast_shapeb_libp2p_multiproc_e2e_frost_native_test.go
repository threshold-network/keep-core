//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	stdnet "net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	keepnet "github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This file is the "shape (B)" real-crypto SEPARATE-PROCESS e2e (the RFC-21 Phase 7.3
// fidelity step that the in-process shape-(A) harness explicitly deferred). Where shape
// (A) runs n runners against ONE process-global engine over the in-process pkg/net/local
// bus - so only the FIRST seat to aggregate wins and the rest observe
// interactive_attempt_already_aggregated - shape (B) launches n SEPARATE OS PROCESSES,
// each with its OWN engine and its OWN encrypted state dir, talking over REAL libp2p
// (gossipsub + the production protobuf/pubsub outer framing, not the local bus). The
// added coverage is exactly the three things shape (A)'s own comment lists as out of
// scope: per-node engine/state isolation, per-process linking/env, and the libp2p outer
// framing - and crucially, because every process has its own engine, EVERY node
// aggregates the BIP-340 signature independently (n winners, not one).
//
// KEY MATERIAL: the engine's DKG is a single centralized FFI call
// (frost_tbtc_run_dkg), so this harness runs DKG ONCE in the orchestrator, which
// persists the encrypted key packages to a bootstrap state file (fixed dev state-
// encryption key), then copies that file into each worker's isolated state dir. Each
// worker's fresh engine loads it and opens the interactive session for its own member.
// (A worker therefore physically holds the whole key group, not just its own share -
// the one fidelity gap vs. an interactive DKG; it is a key-CUSTODY property, not a
// transport/aggregation one, and is orthogonal to what this harness proves. True
// single-share custody needs an interactive DKG or a per-member key-package export FFI,
// neither of which the engine exposes today.)
//
// MECHANISM: the orchestrator re-execs THIS test binary (which is already linked against
// libfrost_tbtc) as each worker via -test.run + the FROST_SHAPEB_WORKER env, so the
// workers inherit the cgo linking with no separate build. Each worker prints
// SHAPEB_SIGNATURE=<hex> to stdout; the orchestrator asserts all n match and are valid
// 64-byte signatures.

const (
	shapeBWorkerEnv      = "FROST_SHAPEB_WORKER"
	shapeBBootstrapEnv   = "FROST_SHAPEB_BOOTSTRAP"
	shapeBConfigEnv      = "FROST_SHAPEB_CONFIG"
	shapeBBootstrapSess  = "FROST_SHAPEB_BS_SESSION"
	shapeBBootstrapN     = "FROST_SHAPEB_BS_N"
	shapeBBootstrapThr   = "FROST_SHAPEB_BS_THRESHOLD"
	shapeBTopic          = "frost-roast-interactive-signing"
	shapeBSigPrefix      = "SHAPEB_SIGNATURE="
	shapeBErrPrefix      = "SHAPEB_ERROR="
	shapeBKeyGroupPrefix = "SHAPEB_KEYGROUP="
)

type shapeBMember struct {
	Index        int    `json:"index"`
	OperatorDHex string `json:"operator_d_hex"`
	Port         int    `json:"port"`
	Multiaddr    string `json:"multiaddr"`
	StatePath    string `json:"state_path"`
}

type shapeBConfig struct {
	N          int            `json:"n"`
	Threshold  int            `json:"threshold"`
	SessionID  string         `json:"session_id"`
	KeyGroup   string         `json:"key_group"`
	MessageHex string         `json:"message_hex"`
	Topic      string         `json:"topic"`
	Members    []shapeBMember `json:"members"`
}

// TestRealCgoInteractiveSigning_Libp2pMultiProc_ShapeB is BOTH the orchestrator and (when
// re-exec'd with FROST_SHAPEB_WORKER set) the per-node worker. The worker branch runs one
// seat to a real signature over real libp2p; the orchestrator branch wires the group,
// launches the workers, and asserts every one independently aggregates the same signature.
func TestRealCgoInteractiveSigning_Libp2pMultiProc_ShapeB(t *testing.T) {
	if os.Getenv(shapeBBootstrapEnv) != "" {
		runShapeBBootstrap(t)
		return
	}
	if idxStr := os.Getenv(shapeBWorkerEnv); idxStr != "" {
		runShapeBWorker(t, idxStr)
		return
	}
	runShapeBOrchestrator(t, 3, 2)
}

// runShapeBBootstrap runs the centralized DKG in its OWN process so the orchestrator
// process never binds the process-global engine (the engine binds its state lock to the
// first path it sees and refuses to switch - which would otherwise collide with the
// in-process shape-(A) tests that run earlier in the same `go test` binary). It persists
// the key group to TBTC_SIGNER_STATE_PATH (set by the parent) and prints the key group.
func runShapeBBootstrap(t *testing.T) {
	n, err := strconv.Atoi(os.Getenv(shapeBBootstrapN))
	if err != nil {
		fmt.Printf("%sbad bootstrap n: %v\n", shapeBErrPrefix, err)
		return
	}
	threshold, err := strconv.Atoi(os.Getenv(shapeBBootstrapThr))
	if err != nil {
		fmt.Printf("%sbad bootstrap threshold: %v\n", shapeBErrPrefix, err)
		return
	}
	sessionID := os.Getenv(shapeBBootstrapSess)
	participantIDs := make([]byte, 0, n)
	for i := 1; i <= n; i++ {
		participantIDs = append(participantIDs, byte(i))
	}
	keyGroup := runRealCgoDKGKeyGroup(t, &buildTaggedTBTCSignerEngine{}, sessionID, participantIDs, uint16(threshold))
	fmt.Printf("%s%s\n", shapeBKeyGroupPrefix, keyGroup)
}

func runShapeBOrchestrator(t *testing.T, n int, threshold uint16) {
	// The SAME fixed dev state-encryption key the in-process harness uses, so the
	// encrypted DKG file the bootstrap process writes can be decrypted by every worker.
	stateKey := make([]byte, 32)
	for i := range stateKey {
		stateKey[i] = byte(i + 1)
	}
	stateKeyHex := hex.EncodeToString(stateKey)

	bootstrapDir := t.TempDir()
	bootstrapState := filepath.Join(bootstrapDir, "signer-state")

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	sessionID := fmt.Sprintf("shapeb-libp2p-%d", realCgoSessionSeq.Add(1))

	// 1. Centralized DKG in a SEPARATE bootstrap process. This keeps the engine out of
	// THIS orchestrator process, which shares the binary - hence the process-global
	// engine and its bound state lock - with the in-process shape-(A) tests that run
	// earlier under the same `-run TestRealCgoInteractiveSigning` invocation.
	bootstrapCmd := exec.CommandContext(ctx, os.Args[0],
		"-test.run", "^TestRealCgoInteractiveSigning_Libp2pMultiProc_ShapeB$",
		"-test.timeout=60s",
	)
	bootstrapCmd.Env = withEnvOverrides(os.Environ(), map[string]string{
		shapeBBootstrapEnv:                     "1",
		shapeBBootstrapSess:                    sessionID,
		shapeBBootstrapN:                       strconv.Itoa(n),
		shapeBBootstrapThr:                     strconv.Itoa(int(threshold)),
		"TBTC_SIGNER_PROFILE":                  "development",
		"TBTC_SIGNER_ENFORCE_PROVENANCE_GATE":  "false",
		"TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX": stateKeyHex,
		"TBTC_SIGNER_STATE_PATH":               bootstrapState,
	})
	bootstrapOut, err := bootstrapCmd.CombinedOutput()
	keyGroup := extractPrefixed(string(bootstrapOut), shapeBKeyGroupPrefix)
	if keyGroup == "" {
		t.Fatalf("bootstrap DKG emitted no key group (err=%v):\n%s", err, indentTail(string(bootstrapOut), 40))
	}
	if fi, statErr := os.Stat(bootstrapState); statErr != nil || fi.Size() == 0 {
		t.Fatalf("bootstrap DKG did not persist a non-empty state at %s (err=%v)", bootstrapState, statErr)
	}

	// 2. One transport operator key per member, free port, and the libp2p peer id (so the
	// peer table - hence every worker's bootstrap Peers list - is known before launch).
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

	members := make([]shapeBMember, 0, n)
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
		stateDir := filepath.Join(t.TempDir(), fmt.Sprintf("member-%d", i+1))
		if err := copyStateDir(bootstrapDir, stateDir); err != nil {
			t.Fatalf("copy state for member %d: %v", i+1, err)
		}
		members = append(members, shapeBMember{
			Index:        i + 1,
			OperatorDHex: hex.EncodeToString(priv.D.Bytes()),
			Port:         port,
			Multiaddr:    fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, peerID),
			StatePath:    filepath.Join(stateDir, "signer-state"),
		})
	}

	messageDigest := make([]byte, attempt.MessageDigestLength)
	for i := range messageDigest {
		messageDigest[i] = 0x42
	}
	cfg := shapeBConfig{
		N:          n,
		Threshold:  int(threshold),
		SessionID:  sessionID,
		KeyGroup:   keyGroup,
		MessageHex: hex.EncodeToString(messageDigest),
		Topic:      shapeBTopic,
		Members:    members,
	}
	configPath := filepath.Join(t.TempDir(), "shapeb-config.json")
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, cfgBytes, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 3. Launch n worker processes (this test binary, re-exec'd) and collect their output.
	type result struct {
		index  int
		output string
		err    error
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	for i := range members {
		wg.Add(1)
		go func(idx int, m shapeBMember) {
			defer wg.Done()
			cmd := exec.CommandContext(ctx, os.Args[0],
				"-test.run", "^TestRealCgoInteractiveSigning_Libp2pMultiProc_ShapeB$",
				"-test.v", "-test.timeout=140s",
			)
			cmd.Env = withEnvOverrides(os.Environ(), map[string]string{
				shapeBWorkerEnv:                        strconv.Itoa(m.Index),
				shapeBConfigEnv:                        configPath,
				"TBTC_SIGNER_PROFILE":                  "development",
				"TBTC_SIGNER_ENFORCE_PROVENANCE_GATE":  "false",
				"TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX": stateKeyHex,
				"TBTC_SIGNER_STATE_PATH":               m.StatePath,
			})
			out, err := cmd.CombinedOutput()
			results[idx] = result{index: m.Index, output: string(out), err: err}
		}(i, members[i])
	}
	wg.Wait()

	// 4. Every worker must independently produce the same valid 64-byte signature.
	var winning string
	winners := 0
	for _, r := range results {
		sig := extractPrefixed(r.output, shapeBSigPrefix)
		if sig == "" {
			t.Fatalf("member %d did not emit a signature (err=%v):\n%s", r.index, r.err, indentTail(r.output, 40))
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
		t.Fatalf("expected all %d separate-process nodes to aggregate the signature, got %d", n, winners)
	}
	t.Logf("shape-B: %d separate-process nodes over real libp2p each aggregated the same BIP-340 signature %s…", n, winning[:16])
}

func runShapeBWorker(t *testing.T, idxStr string) {
	index, err := strconv.Atoi(idxStr)
	if err != nil {
		fmt.Printf("%sbad worker index %q: %v\n", shapeBErrPrefix, idxStr, err)
		return
	}
	cfgBytes, err := os.ReadFile(os.Getenv(shapeBConfigEnv))
	if err != nil {
		fmt.Printf("%sread config: %v\n", shapeBErrPrefix, err)
		return
	}
	var cfg shapeBConfig
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		fmt.Printf("%sparse config: %v\n", shapeBErrPrefix, err)
		return
	}

	var self shapeBMember
	peers := make([]string, 0, cfg.N-1)
	for _, m := range cfg.Members {
		if m.Index == index {
			self = m
		} else {
			peers = append(peers, m.Multiaddr)
		}
	}
	if self.Index == 0 {
		fmt.Printf("%smember %d not found in config\n", shapeBErrPrefix, index)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 130*time.Second)
	defer cancel()

	selfKey, err := operatorKeyFromDHex(self.OperatorDHex)
	if err != nil {
		fmt.Printf("%sreconstruct self key: %v\n", shapeBErrPrefix, err)
		return
	}

	// Real libp2p host: listen on the assigned port, bootstrap to the other members.
	provider, err := libp2p.Connect(
		ctx,
		libp2p.Config{Port: self.Port, Peers: peers, Bootstrap: true},
		selfKey,
		firewall.Disabled,
		periodicRetransmissionTicker(ctx, 300*time.Millisecond),
	)
	if err != nil {
		fmt.Printf("%slibp2p connect: %v\n", shapeBErrPrefix, err)
		return
	}
	if err := waitForPeers(ctx, provider, cfg.N-1, 60*time.Second); err != nil {
		fmt.Printf("%swait for peers: %v\n", shapeBErrPrefix, err)
		return
	}
	// Gossipsub mesh warmup: a connection is not yet a subscribed mesh peer. With the
	// periodic retransmission ticker, early broadcasts still resend until the mesh forms,
	// but a short settle reduces wasted rounds.
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		fmt.Printf("%scontext done during warmup\n", shapeBErrPrefix)
		return
	}

	channel, err := provider.BroadcastChannelFor(cfg.Topic)
	if err != nil {
		fmt.Printf("%sbroadcast channel: %v\n", shapeBErrPrefix, err)
		return
	}

	// Membership: map every member index -> its operator address so the bus authenticates
	// each broadcast's claimed seat against the authenticated libp2p sender key.
	chainSigning := local_v1.Connect(cfg.N, cfg.N).Signing()
	addresses := make([]chain.Address, cfg.N)
	for _, m := range cfg.Members {
		key, err := operatorKeyFromDHex(m.OperatorDHex)
		if err != nil {
			fmt.Printf("%sreconstruct member %d key: %v\n", shapeBErrPrefix, m.Index, err)
			return
		}
		addresses[m.Index-1] = chainSigning.PublicKeyBytesToAddress(
			operator.MarshalUncompressed(&key.PublicKey),
		)
	}
	logger := &testutils.MockLogger{}
	validator := group.NewMembershipValidator(logger, addresses, chainSigning)

	included := make([]group.MemberIndex, 0, cfg.N)
	for i := 1; i <= cfg.N; i++ {
		included = append(included, group.MemberIndex(i))
	}
	keyGroupSeed := []byte(cfg.KeyGroup)
	msgBytes, err := hex.DecodeString(cfg.MessageHex)
	if err != nil || len(msgBytes) != attempt.MessageDigestLength {
		fmt.Printf("%sbad message digest (len=%d err=%v)\n", shapeBErrPrefix, len(msgBytes), err)
		return
	}
	var messageDigest [attempt.MessageDigestLength]byte
	copy(messageDigest[:], msgBytes)

	attemptCtx, err := attempt.NewAttemptContext(
		cfg.SessionID, cfg.KeyGroup, keyGroupSeed, messageDigest, 0, included, nil,
	)
	if err != nil {
		fmt.Printf("%sattempt context: %v\n", shapeBErrPrefix, err)
		return
	}

	member := group.MemberIndex(self.Index)
	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()

	bus, err := NewBroadcastChannelRunnerBus(ctx, logger, channel, validator)
	if err != nil {
		fmt.Printf("%srunner bus: %v\n", shapeBErrPrefix, err)
		return
	}
	coord := roast.NewInMemoryCoordinatorWithSigning(member, signer, verifier)
	handle, err := coord.BeginAttempt(attemptCtx)
	if err != nil {
		fmt.Printf("%sbegin attempt: %v\n", shapeBErrPrefix, err)
		return
	}
	ara, err := NewActiveRoastAttempt(coord, handle, attemptCtx, cfg.SessionID, nil, keyGroupSeed)
	if err != nil {
		fmt.Printf("%sactive attempt: %v\n", shapeBErrPrefix, err)
		return
	}
	collector := roast.NewRound2Collector(verifier)
	// This worker's OWN engine, loaded from its OWN copied state dir.
	engine := &buildTaggedTBTCSignerEngine{}
	runner, err := newInteractiveSigningRunner(
		ara, member, cfg.Threshold2uint16(), engine, collector, coord, signer, bus,
	)
	if err != nil {
		fmt.Printf("%srunner: %v\n", shapeBErrPrefix, err)
		return
	}

	sig, err := runner.Run(ctx)
	if err != nil {
		fmt.Printf("%srun: %v\n", shapeBErrPrefix, err)
		return
	}
	if len(sig) != 64 {
		fmt.Printf("%sunexpected signature length %d\n", shapeBErrPrefix, len(sig))
		return
	}
	state, err := coord.State(handle)
	if err != nil {
		fmt.Printf("%scoordinator state: %v\n", shapeBErrPrefix, err)
		return
	}
	if state != roast.AttemptStateSucceeded {
		fmt.Printf("%sdid not reach Succeeded (got %v)\n", shapeBErrPrefix, state)
		return
	}
	fmt.Printf("%s%s\n", shapeBSigPrefix, hex.EncodeToString(sig))
}

// Threshold2uint16 narrows the JSON int threshold to the uint16 the runner wants.
func (c shapeBConfig) Threshold2uint16() uint16 { return uint16(c.Threshold) }

// ---- helpers ----

func mustGenOperatorKey(t *testing.T) *operator.PrivateKey {
	t.Helper()
	priv, _, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
	if err != nil {
		t.Fatalf("generate operator key: %v", err)
	}
	return priv
}

// operatorKeyFromDHex rebuilds an operator private key from the hex of its scalar D, so
// the orchestrator can hand each worker a stable transport identity across the process
// boundary (the worker's libp2p peer id must match the one in the shared peer table).
func operatorKeyFromDHex(dHex string) (*operator.PrivateKey, error) {
	dBytes, err := hex.DecodeString(dHex)
	if err != nil {
		return nil, fmt.Errorf("decode D: %w", err)
	}
	curve := local_v1.DefaultCurve
	x, y := curve.ScalarBaseMult(dBytes)
	return &operator.PrivateKey{
		PublicKey: operator.PublicKey{Curve: operator.Secp256k1, X: x, Y: y},
		D:         new(big.Int).SetBytes(dBytes),
	}, nil
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*stdnet.TCPAddr).Port
}

// waitForPeers blocks until the provider is connected to at least want peers.
func waitForPeers(ctx context.Context, provider keepnet.Provider, want int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		connected := len(provider.ConnectionManager().ConnectedPeers())
		if connected >= want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("only %d of %d peers connected before timeout", connected, want)
		}
		select {
		case <-time.After(250 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func idleRetransmissionTicker() *retransmission.Ticker {
	ticks := make(chan uint64)
	close(ticks)
	return retransmission.NewTicker(ticks)
}

func periodicRetransmissionTicker(ctx context.Context, interval time.Duration) *retransmission.Ticker {
	ticks := make(chan uint64)
	go func() {
		tk := time.NewTicker(interval)
		defer tk.Stop()
		var n uint64
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				n++
				select {
				case ticks <- n:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return retransmission.NewTicker(ticks)
}

func withEnvOverrides(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		if _, ok := overrides[key]; !ok {
			out = append(out, kv)
		}
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

// copyStateDir copies every non-lock file from src into a fresh dst (the engine binds a
// process-global lock to its own state path, so a stale copied lock must not travel).
func copyStateDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || strings.Contains(e.Name(), ".lock") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func extractPrefixed(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func indentTail(output string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return "    " + strings.Join(lines, "\n    ")
}
