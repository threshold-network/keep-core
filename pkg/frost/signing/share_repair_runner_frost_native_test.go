//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type testShareRepairEngine struct {
	mutex        sync.Mutex
	beginCalls   int
	finishCalls  int
	installCalls int
}

type testShareRepairSessionOverrideEngine struct {
	*testShareRepairEngine
	beginError  error
	finishError error
	mutate      func(*NativeShareRepairSession)
}

type captureShareRepairPart2Engine struct {
	*testShareRepairEngine
	deltas []*NativeShareRepairEncryptedDelta
}

func (engine *captureShareRepairPart2Engine) ShareRepairPart2(
	_ *ShareRepairAuthorization,
	_ uint16,
	deltas []*NativeShareRepairEncryptedDelta,
	_ *ShareRepairTransportRoster,
) (*NativeShareRepairPart2Result, error) {
	engine.deltas = append([]*NativeShareRepairEncryptedDelta(nil), deltas...)
	return nil, fmt.Errorf("stop after part2 input")
}

type captureShareRepairInstallEngine struct {
	*testShareRepairEngine
	sigmas []*NativeShareRepairEncryptedSigma
}

func (engine *captureShareRepairInstallEngine) InstallRepairedShare(
	_ *ShareRepairAuthorization,
	_ *NativeFROSTPublicKeyPackage,
	sigmas []*NativeShareRepairEncryptedSigma,
	_ *ShareRepairTransportRoster,
) (*NativeShareRepairInstallResult, error) {
	engine.sigmas = append([]*NativeShareRepairEncryptedSigma(nil), sigmas...)
	return nil, fmt.Errorf("stop after install input")
}

func (engine *testShareRepairSessionOverrideEngine) BeginShareRepairSession(
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) (*NativeShareRepairSession, error) {
	if engine.beginError != nil {
		engine.mutex.Lock()
		engine.beginCalls++
		engine.mutex.Unlock()
		return nil, engine.beginError
	}
	session, err := engine.testShareRepairEngine.BeginShareRepairSession(
		authorization,
		participantIdentifier,
	)
	if session != nil && engine.mutate != nil {
		engine.mutate(session)
	}
	return session, err
}

func (engine *testShareRepairSessionOverrideEngine) FinishShareRepairSession(
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) error {
	_ = engine.testShareRepairEngine.FinishShareRepairSession(
		authorization,
		participantIdentifier,
	)
	return engine.finishError
}

const testShareRepairCiphertextLength = shareRepairEncryptedScalarPayloadLength

func testShareRepairSecret(value byte) []byte {
	return bytes.Repeat([]byte{value}, 32)
}

func testShareRepairPublicKeyPackage() *NativeFROSTPublicKeyPackage {
	return &NativeFROSTPublicKeyPackage{
		VerifyingShares: map[string]string{
			"1": "share-1",
			"2": "share-2",
			"3": "share-3",
		},
		VerifyingKey: "group-verifying-key",
	}
}

func testShareRepairPublicKey(identifier uint16) []byte {
	privateBytes := bytes.Repeat([]byte{byte(0x11 * identifier)}, 32)
	_, publicKey := btcec.PrivKeyFromBytes(privateBytes)
	return publicKey.SerializeCompressed()
}

func testShareRepairCiphertext(kind byte, sender, recipient uint16) []byte {
	result := bytes.Repeat([]byte{0xa5}, testShareRepairCiphertextLength)
	result[0] = kind
	result[1] = byte(sender >> 8)
	result[2] = byte(sender)
	result[3] = byte(recipient >> 8)
	result[4] = byte(recipient)
	return result
}

func currentTestShareRepairAuthorization(
	t *testing.T,
) (*ShareRepairAuthorization, ed25519.PrivateKey) {
	t.Helper()
	authorization, authority := testShareRepairAuthorization(t)
	now := uint64(time.Now().Unix())
	authorization.IssuedAtUnix = now - 60
	// Preflight is intentionally allowed before the recovery not-before time.
	authorization.NotBeforeUnix = now + 300
	authorization.ExpiresAtUnix = now + 3600
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	authorization.SignatureHex = "0x" + hex.EncodeToString(
		ed25519.Sign(authority, digest[:]),
	)
	return authorization, authority
}

func (engine *testShareRepairEngine) BeginShareRepairSession(
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) (*NativeShareRepairSession, error) {
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return nil, err
	}
	engine.mutex.Lock()
	engine.beginCalls++
	engine.mutex.Unlock()
	return &NativeShareRepairSession{
		ContextDigest:         fmt.Sprintf("0x%x", digest),
		ParticipantIdentifier: participantIdentifier,
		StoreFingerprint:      authorization.NewStoreFingerprint,
		TransportPublicKey:    testShareRepairPublicKey(participantIdentifier),
	}, nil
}

func (engine *testShareRepairEngine) FinishShareRepairSession(
	_ *ShareRepairAuthorization,
	_ uint16,
) error {
	engine.mutex.Lock()
	engine.finishCalls++
	engine.mutex.Unlock()
	return nil
}

func (engine *testShareRepairEngine) ShareRepairPart1(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	transportRoster *ShareRepairTransportRoster,
) (*NativeShareRepairPart1Result, error) {
	if _, err := ComputeShareRepairTransportRosterDigest(
		transportRoster,
		authorization,
	); err != nil {
		return nil, fmt.Errorf("invalid transport roster: %w", err)
	}
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return nil, err
	}
	contextWire := fmt.Sprintf("0x%x", digest)
	result := &NativeShareRepairPart1Result{
		ContextDigest:    contextWire,
		HelperIdentifier: helperIdentifier,
		PublicKeyPackage: testShareRepairPublicKeyPackage(),
	}
	for index, recipient := range authorization.HelperIdentifiers {
		endpoint := transportRoster.ParticipantPublicKeys[index]
		publicKey, _ := hex.DecodeString(endpoint.PublicKeyHex)
		if endpoint.ParticipantIdentifier != recipient ||
			!bytes.Equal(publicKey, testShareRepairPublicKey(recipient)) {
			return nil, fmt.Errorf("wrong recipient key [%d]", index)
		}
		result.Deltas = append(result.Deltas, &NativeShareRepairEncryptedDelta{
			ContextDigest:       contextWire,
			SenderIdentifier:    helperIdentifier,
			RecipientIdentifier: recipient,
			Payload:             testShareRepairCiphertext(1, helperIdentifier, recipient),
		})
	}
	return result, nil
}

func (engine *testShareRepairEngine) ShareRepairPart2(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	deltas []*NativeShareRepairEncryptedDelta,
	transportRoster *ShareRepairTransportRoster,
) (*NativeShareRepairPart2Result, error) {
	if _, err := ComputeShareRepairTransportRosterDigest(
		transportRoster,
		authorization,
	); err != nil {
		return nil, fmt.Errorf("invalid transport roster: %w", err)
	}
	if len(deltas) != len(authorization.HelperIdentifiers) {
		return nil, fmt.Errorf("wrong delta count")
	}
	for index, sender := range authorization.HelperIdentifiers {
		delta := deltas[index]
		if delta == nil || delta.SenderIdentifier != sender ||
			delta.RecipientIdentifier != helperIdentifier ||
			!bytes.Equal(delta.Payload, testShareRepairCiphertext(1, sender, helperIdentifier)) {
			return nil, fmt.Errorf("wrong delta [%d]", index)
		}
	}
	targetPublicKey, _ := hex.DecodeString(
		transportRoster.ParticipantPublicKeys[len(transportRoster.ParticipantPublicKeys)-1].PublicKeyHex,
	)
	if !bytes.Equal(
		targetPublicKey,
		testShareRepairPublicKey(authorization.TargetIdentifier),
	) {
		return nil, fmt.Errorf("wrong target public key")
	}
	digest, _ := ComputeShareRepairAuthorizationDigest(authorization)
	contextWire := fmt.Sprintf("0x%x", digest)
	return &NativeShareRepairPart2Result{
		ContextDigest: contextWire,
		Sigma: &NativeShareRepairEncryptedSigma{
			ContextDigest:    contextWire,
			HelperIdentifier: helperIdentifier,
			Payload: testShareRepairCiphertext(
				2,
				helperIdentifier,
				authorization.TargetIdentifier,
			),
		},
	}, nil
}

func (engine *testShareRepairEngine) InstallRepairedShare(
	authorization *ShareRepairAuthorization,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
	sigmas []*NativeShareRepairEncryptedSigma,
	transportRoster *ShareRepairTransportRoster,
) (*NativeShareRepairInstallResult, error) {
	if _, err := ComputeShareRepairTransportRosterDigest(
		transportRoster,
		authorization,
	); err != nil {
		return nil, fmt.Errorf("invalid transport roster: %w", err)
	}
	if publicKeyPackage == nil || publicKeyPackage.VerifyingKey != "group-verifying-key" ||
		len(publicKeyPackage.VerifyingShares) != 3 {
		return nil, fmt.Errorf("wrong public key package")
	}
	if len(sigmas) != len(authorization.HelperIdentifiers) {
		return nil, fmt.Errorf("wrong sigma count")
	}
	for index, helper := range authorization.HelperIdentifiers {
		sigma := sigmas[index]
		if sigma == nil || sigma.HelperIdentifier != helper ||
			!bytes.Equal(
				sigma.Payload,
				testShareRepairCiphertext(2, helper, authorization.TargetIdentifier),
			) {
			return nil, fmt.Errorf("wrong sigma [%d]", index)
		}
	}
	engine.mutex.Lock()
	engine.installCalls++
	engine.mutex.Unlock()
	digest, _ := ComputeShareRepairAuthorizationDigest(authorization)
	return &NativeShareRepairInstallResult{
		Schema:                 ShareRepairInstallResultSchema,
		SessionID:              authorization.SessionID,
		KeyGroup:               authorization.KeyGroup,
		TargetIdentifier:       authorization.TargetIdentifier,
		RecoveryEpoch:          authorization.RecoveryEpoch,
		AuthorizationDigest:    fmt.Sprintf("0x%x", digest),
		ActiveStoreFingerprint: authorization.NewStoreFingerprint,
	}, nil
}

type recordingShareRepairBus struct {
	shareRepairBus
	mutex    sync.Mutex
	messages []shareRepairMessage
}

func (bus *recordingShareRepairBus) Broadcast(
	message shareRepairMessage,
) context.CancelFunc {
	copy := message
	copy.EphemeralPublicKey = append([]byte(nil), message.EphemeralPublicKey...)
	copy.Payload = append([]byte(nil), message.Payload...)
	bus.mutex.Lock()
	bus.messages = append(bus.messages, copy)
	bus.mutex.Unlock()
	return bus.shareRepairBus.Broadcast(message)
}

type subscriptionBarrierShareRepairBus struct {
	shareRepairBus
	expected int
	mutex    sync.Mutex
	count    int
	ready    chan struct{}
}

func (bus *subscriptionBarrierShareRepairBus) Subscribe(
	member group.MemberIndex,
) <-chan shareRepairMessage {
	stream := bus.shareRepairBus.Subscribe(member)
	bus.mutex.Lock()
	bus.count++
	if bus.count == bus.expected {
		close(bus.ready)
	}
	bus.mutex.Unlock()
	return stream
}

func (bus *subscriptionBarrierShareRepairBus) Broadcast(
	message shareRepairMessage,
) context.CancelFunc {
	<-bus.ready
	return bus.shareRepairBus.Broadcast(message)
}

func TestRunShareRepairOnBusConfidentialExactSetAndInstall(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	engine := &testShareRepairEngine{}
	bus := &recordingShareRepairBus{
		shareRepairBus: &subscriptionBarrierShareRepairBus{
			shareRepairBus: newInProcessShareRepairBus(128),
			expected:       3,
			ready:          make(chan struct{}),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type outcome struct {
		member group.MemberIndex
		result *NativeShareRepairInstallResult
		err    error
	}
	outcomes := make(chan outcome, 3)
	for _, member := range []group.MemberIndex{1, 2, 3} {
		member := member
		go func() {
			result, err := runShareRepairOnBus(
				ctx,
				engine,
				authorization,
				transportRoster,
				digest,
				[]group.MemberIndex{member},
				bus,
			)
			outcomes <- outcome{member: member, result: result, err: err}
		}()
	}
	targetResults := 0
	for i := 0; i < 3; i++ {
		outcome := <-outcomes
		if outcome.err != nil {
			t.Fatalf("seat [%d] failed: %v", outcome.member, outcome.err)
		}
		if outcome.result != nil {
			if outcome.member != 3 {
				t.Fatalf("helper seat [%d] returned an install result", outcome.member)
			}
			targetResults++
		}
	}
	if targetResults != 1 {
		t.Fatalf("expected one target install result, got [%d]", targetResults)
	}
	engine.mutex.Lock()
	if engine.installCalls != 1 {
		engine.mutex.Unlock()
		t.Fatalf("expected one native install, got [%d]", engine.installCalls)
	}
	if engine.beginCalls != 3 || engine.finishCalls != 3 {
		engine.mutex.Unlock()
		t.Fatalf(
			"expected three native session begin/finish calls, got [%d]/[%d]",
			engine.beginCalls,
			engine.finishCalls,
		)
	}
	engine.mutex.Unlock()

	bus.mutex.Lock()
	defer bus.mutex.Unlock()
	secretMessages := 0
	publicPackages := 0
	installedAcknowledgements := 0
	completions := 0
	for _, message := range bus.messages {
		if message.Type == shareRepairAnnouncementMessage {
			if len(message.Payload) != 0 || len(message.EphemeralPublicKey) != 33 {
				t.Fatal("public recovery announcement has the wrong shape")
			}
			continue
		}
		if message.Type == shareRepairInstalledMessage {
			if len(message.Payload) == 0 || message.Recipient != 0 || message.Sender != 3 {
				t.Fatal("installed receipt has the wrong public shape")
			}
			continue
		}
		if message.Type == shareRepairPublicPackageMessage {
			publicPackages++
			if message.Recipient != 0 || len(message.Payload) == 0 {
				t.Fatal("public key package has the wrong public shape")
			}
			continue
		}
		if message.Type == shareRepairInstalledAcknowledgementMessage {
			installedAcknowledgements++
			if message.Recipient != 3 || len(message.Payload) != sha256.Size ||
				(message.Sender != 1 && message.Sender != 2) {
				t.Fatal("installed receipt acknowledgement has the wrong public shape")
			}
			continue
		}
		if message.Type == shareRepairCompletionMessage {
			completions++
			if message.Recipient != 0 || message.Sender != 3 ||
				len(message.Payload) != sha256.Size {
				t.Fatal("share-repair completion has the wrong public shape")
			}
			continue
		}
		secretMessages++
		if bytes.Contains(message.Payload, []byte("data_hex")) {
			t.Fatal("repair scalar was sent as plaintext JSON")
		}
		for _, value := range []byte{11, 12, 21, 22, 101, 102} {
			plaintextHex := []byte(hex.EncodeToString(testShareRepairSecret(value)))
			if bytes.Contains(message.Payload, plaintextHex) {
				t.Fatalf("repair scalar [%d] appears in the network payload", value)
			}
		}
	}
	if secretMessages != 6 {
		t.Fatalf("expected four deltas and two sigmas, got [%d] secret messages", secretMessages)
	}
	if publicPackages != 2 {
		t.Fatalf("expected one public package from each helper, got [%d]", publicPackages)
	}
	if installedAcknowledgements != 2 {
		t.Fatalf(
			"expected one installed receipt acknowledgement from each helper, got [%d]",
			installedAcknowledgements,
		)
	}
	if completions != 1 {
		t.Fatalf("expected one share-repair completion, got [%d]", completions)
	}
}

func TestRunShareRepairOnBusTimesOutWithoutExactParticipantSet(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, _ := ComputeShareRepairAuthorizationDigest(authorization)
	bus := &recordingShareRepairBus{shareRepairBus: newInProcessShareRepairBus(16)}
	engine := &testShareRepairEngine{}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	_, err := runShareRepairOnBus(
		ctx,
		engine,
		authorization,
		transportRoster,
		digest,
		[]group.MemberIndex{1},
		bus,
	)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("context deadline exceeded")) {
		t.Fatalf("expected exact-set announcement timeout, got [%v]", err)
	}
	engine.mutex.Lock()
	if engine.beginCalls != 1 || engine.finishCalls != 1 {
		engine.mutex.Unlock()
		t.Fatalf(
			"native timeout cleanup was incomplete: begin [%d], finish [%d]",
			engine.beginCalls,
			engine.finishCalls,
		)
	}
	engine.mutex.Unlock()
	bus.mutex.Lock()
	defer bus.mutex.Unlock()
	announcementCount := 0
	for _, message := range bus.messages {
		if message.Type == shareRepairAnnouncementMessage {
			announcementCount++
		}
	}
	if announcementCount != 1 {
		t.Fatalf(
			"application layer must publish one announcement, got [%d]",
			announcementCount,
		)
	}
}

func TestPrepareShareRepairTransportRosterEntry(t *testing.T) {
	authorization, authority := currentTestShareRepairAuthorization(t)
	authorityPublicKey := authority.Public().(ed25519.PublicKey)

	t.Run("success before not-before", func(t *testing.T) {
		engine := &testShareRepairEngine{}
		entry, err := PrepareShareRepairTransportRosterEntry(
			engine,
			authorization,
			authorityPublicKey,
			1,
		)
		if err != nil {
			t.Fatal(err)
		}
		if entry.ParticipantIdentifier != 1 ||
			entry.StoreFingerprint != authorization.NewStoreFingerprint ||
			entry.PublicKeyHex != hex.EncodeToString(testShareRepairPublicKey(1)) {
			t.Fatalf("unexpected transport preflight entry: %+v", entry)
		}
		engine.mutex.Lock()
		defer engine.mutex.Unlock()
		if engine.beginCalls != 1 || engine.finishCalls != 1 {
			t.Fatalf(
				"transport preflight lifecycle was [%d]/[%d], expected 1/1",
				engine.beginCalls,
				engine.finishCalls,
			)
		}
	})

	t.Run("begin error still cleans up", func(t *testing.T) {
		engine := &testShareRepairSessionOverrideEngine{
			testShareRepairEngine: &testShareRepairEngine{},
			beginError:            fmt.Errorf("response decode failed after native begin"),
		}
		if _, err := PrepareShareRepairTransportRosterEntry(
			engine,
			authorization,
			authorityPublicKey,
			1,
		); err == nil {
			t.Fatal("transport preflight accepted a failed native begin")
		}
		engine.mutex.Lock()
		defer engine.mutex.Unlock()
		if engine.beginCalls != 1 || engine.finishCalls != 1 {
			t.Fatalf("failed begin cleanup was [%d]/[%d]", engine.beginCalls, engine.finishCalls)
		}
	})

	for name, mutate := range map[string]func(*NativeShareRepairSession){
		"invalid public key": func(session *NativeShareRepairSession) {
			session.TransportPublicKey = bytes.Repeat([]byte{0xff}, shareRepairEphemeralPublicKeyLength)
		},
		"invalid store fingerprint": func(session *NativeShareRepairSession) {
			session.StoreFingerprint = "0x00"
		},
	} {
		t.Run(name+" cleans up", func(t *testing.T) {
			engine := &testShareRepairSessionOverrideEngine{
				testShareRepairEngine: &testShareRepairEngine{},
				mutate:                mutate,
			}
			if _, err := PrepareShareRepairTransportRosterEntry(
				engine,
				authorization,
				authorityPublicKey,
				1,
			); err == nil {
				t.Fatal("transport preflight accepted an invalid native session")
			}
			engine.mutex.Lock()
			defer engine.mutex.Unlock()
			if engine.beginCalls != 1 || engine.finishCalls != 1 {
				t.Fatalf("invalid session cleanup was [%d]/[%d]", engine.beginCalls, engine.finishCalls)
			}
		})
	}

	t.Run("finish error is reported", func(t *testing.T) {
		engine := &testShareRepairSessionOverrideEngine{
			testShareRepairEngine: &testShareRepairEngine{},
			finishError:           fmt.Errorf("native cleanup failed"),
		}
		if _, err := PrepareShareRepairTransportRosterEntry(
			engine,
			authorization,
			authorityPublicKey,
			1,
		); err == nil || !bytes.Contains([]byte(err.Error()), []byte("cleanup failed")) {
			t.Fatalf("transport preflight hid finish failure: %v", err)
		}
	})

	t.Run("unauthorized seat is rejected before begin", func(t *testing.T) {
		engine := &testShareRepairEngine{}
		if _, err := PrepareShareRepairTransportRosterEntry(
			engine,
			authorization,
			authorityPublicKey,
			99,
		); err == nil {
			t.Fatal("transport preflight accepted an unauthorized seat")
		}
		engine.mutex.Lock()
		defer engine.mutex.Unlock()
		if engine.beginCalls != 0 || engine.finishCalls != 0 {
			t.Fatal("unauthorized seat reached the native engine")
		}
	})

	t.Run("native-invalid session id is rejected before begin", func(t *testing.T) {
		candidate := *authorization
		candidate.SessionID = "repair wallet"
		engine := &testShareRepairEngine{}
		_, err := PrepareShareRepairTransportRosterEntry(
			engine,
			&candidate,
			authorityPublicKey,
			1,
		)
		if err == nil || !strings.Contains(err.Error(), "session id is invalid") {
			t.Fatalf("transport preflight did not enforce the native session ID contract: %v", err)
		}
		engine.mutex.Lock()
		defer engine.mutex.Unlock()
		if engine.beginCalls != 0 || engine.finishCalls != 0 {
			t.Fatal("native-invalid session ID reached the native engine")
		}
	})

	for name, timestamp := range map[string]func(*ShareRepairAuthorization){
		"before issued": func(candidate *ShareRepairAuthorization) {
			candidate.IssuedAtUnix = uint64(time.Now().Unix()) + 60
			candidate.NotBeforeUnix = candidate.IssuedAtUnix
			candidate.ExpiresAtUnix = candidate.IssuedAtUnix + 60
		},
		"expired": func(candidate *ShareRepairAuthorization) {
			candidate.IssuedAtUnix = 1
			candidate.NotBeforeUnix = 1
			candidate.ExpiresAtUnix = 2
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := *authorization
			timestamp(&candidate)
			digest, err := ComputeShareRepairAuthorizationDigest(&candidate)
			if err != nil {
				t.Fatal(err)
			}
			candidate.SignatureHex = "0x" + hex.EncodeToString(
				ed25519.Sign(authority, digest[:]),
			)
			engine := &testShareRepairEngine{}
			if _, err := PrepareShareRepairTransportRosterEntry(
				engine,
				&candidate,
				authorityPublicKey,
				1,
			); err == nil {
				t.Fatalf("transport preflight accepted authorization %s", name)
			}
		})
	}
}

func TestRunShareRepairRejectsNativeSessionOutsideSignedRoster(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]func(*NativeShareRepairSession){
		"public key": func(session *NativeShareRepairSession) {
			session.TransportPublicKey = testShareRepairPublicKey(2)
		},
		"store fingerprint": func(session *NativeShareRepairSession) {
			session.StoreFingerprint = testShareRepairHex32(0x99)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			base := &testShareRepairEngine{}
			engine := &testShareRepairSessionOverrideEngine{
				testShareRepairEngine: base,
				mutate:                mutate,
			}
			_, err := runShareRepairOnBus(
				context.Background(),
				engine,
				authorization,
				transportRoster,
				digest,
				[]group.MemberIndex{1},
				newInProcessShareRepairBus(4),
			)
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte("signed")) {
				t.Fatalf("native %s mismatch was accepted: %v", name, err)
			}
			base.mutex.Lock()
			defer base.mutex.Unlock()
			if base.beginCalls != 1 || base.finishCalls != 1 {
				t.Fatalf("mismatch lifecycle was [%d]/[%d]", base.beginCalls, base.finishCalls)
			}
		})
	}
}

type manualShareRepairBus struct {
	broadcasts chan shareRepairMessage
}

func (*manualShareRepairBus) Subscribe(group.MemberIndex) <-chan shareRepairMessage {
	return nil
}

func (*manualShareRepairBus) Start() {}

func (bus *manualShareRepairBus) Broadcast(
	message shareRepairMessage,
) context.CancelFunc {
	bus.broadcasts <- message
	return func() {}
}

type cancelTrackingShareRepairBroadcast struct {
	message  shareRepairMessage
	canceled chan struct{}
	once     sync.Once
}

type cancelTrackingShareRepairBus struct {
	broadcasts chan *cancelTrackingShareRepairBroadcast
}

func (*cancelTrackingShareRepairBus) Subscribe(
	group.MemberIndex,
) <-chan shareRepairMessage {
	return nil
}

func (*cancelTrackingShareRepairBus) Start() {}

func (bus *cancelTrackingShareRepairBus) Broadcast(
	message shareRepairMessage,
) context.CancelFunc {
	broadcast := &cancelTrackingShareRepairBroadcast{
		message:  message,
		canceled: make(chan struct{}),
	}
	bus.broadcasts <- broadcast
	return func() { broadcast.once.Do(func() { close(broadcast.canceled) }) }
}

func assertShareRepairStillWaiting(
	t *testing.T,
	result <-chan error,
	stage string,
) {
	t.Helper()
	select {
	case err := <-result:
		t.Fatalf("protocol returned before %s: [%v]", stage, err)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestShareRepairAnnouncementRemainsLivePastLocalRendezvous(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	bus := &cancelTrackingShareRepairBus{
		broadcasts: make(chan *cancelTrackingShareRepairBroadcast, 8),
	}
	stream := make(chan shareRepairMessage, 4)
	runner := &shareRepairRunner{
		member:              1,
		authorization:       authorization,
		transportRoster:     transportRoster,
		authorizationDigest: digest,
		contextWire:         fmt.Sprintf("0x%x", digest),
		participants: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
			3: {},
		},
		helperSet: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		engine:          &testShareRepairEngine{},
		bus:             bus,
		stream:          stream,
		ephemeralPublic: testShareRepairPublicKey(1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		_, err := runner.run(ctx)
		completed <- err
	}()

	announcement := <-bus.broadcasts
	if announcement.message.Type != shareRepairAnnouncementMessage {
		t.Fatalf("runner first published the wrong message: %+v", announcement.message)
	}
	for _, peer := range []group.MemberIndex{2, 3} {
		stream <- shareRepairMessage{
			Type:               shareRepairAnnouncementMessage,
			Sender:             peer,
			ContextDigest:      digest,
			EphemeralPublicKey: testShareRepairPublicKey(uint16(peer)),
		}
	}

	for {
		broadcast := <-bus.broadcasts
		if broadcast.message.Type == shareRepairPublicPackageMessage {
			break
		}
	}
	select {
	case <-announcement.canceled:
		t.Fatal("runner canceled its announcement at only a local rendezvous")
	default:
	}
	cancel()
	select {
	case err := <-completed:
		if err == nil || !bytes.Contains([]byte(err.Error()), []byte("context canceled")) {
			t.Fatalf("runner did not stop on cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop after cancellation")
	}
	select {
	case <-announcement.canceled:
	default:
		t.Fatal("runner did not cancel its announcement when the protocol ended")
	}
}

func TestShareRepairTargetRetainsReceiptUntilExactHelperAcknowledgements(t *testing.T) {
	authorization, _ := testShareRepairAuthorization(t)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	bus := &manualShareRepairBus{broadcasts: make(chan shareRepairMessage, 4)}
	stream := make(chan shareRepairMessage, 8)
	runner := &shareRepairRunner{
		member:              group.MemberIndex(authorization.TargetIdentifier),
		authorization:       authorization,
		authorizationDigest: digest,
		contextWire:         fmt.Sprintf("0x%x", digest),
		helperSet: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		bus:    bus,
		stream: stream,
	}
	installResult := &NativeShareRepairInstallResult{
		Schema:                 ShareRepairInstallResultSchema,
		SessionID:              authorization.SessionID,
		KeyGroup:               authorization.KeyGroup,
		TargetIdentifier:       authorization.TargetIdentifier,
		RecoveryEpoch:          authorization.RecoveryEpoch,
		AuthorizationDigest:    runner.contextWire,
		ActiveStoreFingerprint: authorization.NewStoreFingerprint,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		completed <- runner.publishInstalledReceiptAndWaitForAcknowledgements(
			ctx,
			installResult,
		)
	}()

	var receipt shareRepairMessage
	select {
	case receipt = <-bus.broadcasts:
	case <-ctx.Done():
		t.Fatal("target did not publish the installed receipt")
	}
	if receipt.Type != shareRepairInstalledMessage || receipt.Sender != runner.member {
		t.Fatalf("target published the wrong installed receipt: %+v", receipt)
	}
	receiptDigest := sha256.Sum256(receipt.Payload)
	assertShareRepairStillWaiting(t, completed, "any helper acknowledgement")

	stream <- shareRepairMessage{
		Type:          shareRepairInstalledAcknowledgementMessage,
		Sender:        2,
		Recipient:     runner.member,
		ContextDigest: [32]byte{0xff},
		Payload:       append([]byte(nil), receiptDigest[:]...),
	}
	stream <- shareRepairMessage{
		Type:          shareRepairInstalledAcknowledgementMessage,
		Sender:        1,
		Recipient:     runner.member,
		ContextDigest: digest,
		Payload:       append([]byte(nil), receiptDigest[:]...),
	}
	assertShareRepairStillWaiting(t, completed, "the exact helper set")

	stream <- shareRepairMessage{
		Type:          shareRepairInstalledAcknowledgementMessage,
		Sender:        2,
		Recipient:     runner.member,
		ContextDigest: digest,
		Payload:       append([]byte(nil), receiptDigest[:]...),
	}
	select {
	case err := <-completed:
		if err != nil {
			t.Fatalf("target rejected the exact acknowledgement set: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("target did not finish after the exact acknowledgement set")
	}
	select {
	case completion := <-bus.broadcasts:
		if completion.Type != shareRepairCompletionMessage ||
			completion.Sender != runner.member || completion.Recipient != 0 ||
			completion.ContextDigest != digest ||
			!bytes.Equal(completion.Payload, receiptDigest[:]) {
			t.Fatalf("target published the wrong completion: %+v", completion)
		}
	case <-ctx.Done():
		t.Fatal("target did not publish the share-repair completion")
	}
	select {
	case duplicate := <-bus.broadcasts:
		t.Fatalf("target published an extra receipt or completion: %+v", duplicate)
	default:
	}
}

func TestShareRepairHelperRetainsAcknowledgementUntilTargetCompletion(t *testing.T) {
	authorization, _ := testShareRepairAuthorization(t)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	bus := &manualShareRepairBus{broadcasts: make(chan shareRepairMessage, 2)}
	stream := make(chan shareRepairMessage, 4)
	runner := &shareRepairRunner{
		member:              1,
		authorization:       authorization,
		authorizationDigest: digest,
		contextWire:         fmt.Sprintf("0x%x", digest),
		bus:                 bus,
		stream:              stream,
	}
	receipt, err := json.Marshal(shareRepairInstalledWire{
		Schema:                 ShareRepairInstallResultSchema,
		SessionID:              authorization.SessionID,
		KeyGroup:               authorization.KeyGroup,
		TargetIdentifier:       authorization.TargetIdentifier,
		RecoveryEpoch:          authorization.RecoveryEpoch,
		AuthorizationDigest:    runner.contextWire,
		ActiveStoreFingerprint: authorization.NewStoreFingerprint,
	})
	if err != nil {
		t.Fatal(err)
	}
	receiptDigest := sha256.Sum256(receipt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	completed := make(chan error, 1)
	preReceiptCanceled := make(chan struct{})
	var cancelPreReceiptOnce sync.Once
	go func() {
		completed <- runner.waitForInstalledReceipt(ctx, func() {
			cancelPreReceiptOnce.Do(func() { close(preReceiptCanceled) })
		})
	}()
	stream <- shareRepairMessage{
		Type:          shareRepairInstalledMessage,
		Sender:        group.MemberIndex(authorization.TargetIdentifier),
		ContextDigest: digest,
		Payload:       receipt,
	}

	select {
	case acknowledgement := <-bus.broadcasts:
		if acknowledgement.Type != shareRepairInstalledAcknowledgementMessage ||
			acknowledgement.Sender != runner.member ||
			acknowledgement.Recipient != group.MemberIndex(authorization.TargetIdentifier) ||
			acknowledgement.ContextDigest != digest ||
			!bytes.Equal(acknowledgement.Payload, receiptDigest[:]) {
			t.Fatalf("helper published the wrong acknowledgement: %+v", acknowledgement)
		}
	case <-ctx.Done():
		t.Fatal("helper did not acknowledge the installed receipt")
	}
	select {
	case <-preReceiptCanceled:
	default:
		t.Fatal("helper retained pre-receipt retransmitters after validating the receipt")
	}
	assertShareRepairStillWaiting(t, completed, "target completion")

	stream <- shareRepairMessage{
		Type:          shareRepairCompletionMessage,
		Sender:        group.MemberIndex(authorization.TargetIdentifier),
		ContextDigest: [32]byte{0xff},
		Payload:       append([]byte(nil), receiptDigest[:]...),
	}
	assertShareRepairStillWaiting(t, completed, "matching target completion")
	stream <- shareRepairMessage{
		Type:          shareRepairCompletionMessage,
		Sender:        group.MemberIndex(authorization.TargetIdentifier),
		ContextDigest: digest,
		Payload:       append([]byte(nil), receiptDigest[:]...),
	}
	select {
	case err := <-completed:
		if err != nil {
			t.Fatalf("helper rejected the matching target completion: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("helper did not finish after target completion")
	}
}

func TestShareRepairHelperRelayDeadlineAfterReceiptIsSuccessful(t *testing.T) {
	stream := make(chan shareRepairMessage)
	runner := &shareRepairRunner{stream: stream}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		completed <- runner.waitForShareRepairCompletion(ctx, [sha256.Size]byte{0x01})
	}()
	assertShareRepairStillWaiting(t, completed, "the acknowledgement relay deadline")
	select {
	case err := <-completed:
		if err != nil {
			t.Fatalf("validated receipt became a failure at the relay deadline: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("helper did not finish at the acknowledgement relay deadline")
	}

	canceledContext, cancelImmediately := context.WithCancel(context.Background())
	cancelImmediately()
	if err := runner.waitForShareRepairCompletion(
		canceledContext,
		[sha256.Size]byte{0x01},
	); err == nil || !bytes.Contains([]byte(err.Error()), []byte("context canceled")) {
		t.Fatalf("external cancellation was not reported: %v", err)
	}
}

func TestShareRepairRunnerRejectsPendingMessageFlood(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	stream := make(chan shareRepairMessage, shareRepairMaximumPendingMessages+1)
	for index := 0; index <= shareRepairMaximumPendingMessages; index++ {
		stream <- shareRepairMessage{
			Type:          shareRepairDeltaMessage,
			Sender:        2,
			Recipient:     1,
			ContextDigest: digest,
			Payload:       []byte{byte(index)},
		}
	}
	runner := &shareRepairRunner{
		member:              1,
		authorizationDigest: digest,
		transportRoster:     transportRoster,
		participants: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		stream:          stream,
		ephemeralPublic: testShareRepairPublicKey(1),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = runner.collectAnnouncements(ctx)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("pending-message limit")) {
		t.Fatalf("expected pending-message flood rejection, got [%v]", err)
	}
}

func TestShareRepairRunnerRejectsPendingByteFloodBeforeCountLimit(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	stream := make(chan shareRepairMessage, 3)
	stream <- shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        2,
		ContextDigest: digest,
		Payload:       bytes.Repeat([]byte{0x41}, shareRepairMaximumPublicPayload),
	}
	stream <- shareRepairMessage{
		Type:          shareRepairInstalledMessage,
		Sender:        2,
		ContextDigest: digest,
		Payload:       bytes.Repeat([]byte{0x42}, shareRepairMaximumSecretPayload),
	}
	stream <- shareRepairMessage{
		Type:          shareRepairDeltaMessage,
		Sender:        2,
		Recipient:     1,
		ContextDigest: digest,
		Payload:       testShareRepairCiphertext(1, 2, 1),
	}
	runner := &shareRepairRunner{
		member:              1,
		authorizationDigest: digest,
		transportRoster:     transportRoster,
		participants: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		stream:          stream,
		ephemeralPublic: testShareRepairPublicKey(1),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = runner.collectAnnouncements(ctx)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("pending-byte limit")) {
		t.Fatalf("expected pending-byte flood rejection, got [%v]", err)
	}
	if len(runner.pending) != 2 || runner.pendingBytes != shareRepairMaximumSessionBytesPerSender {
		t.Fatalf(
			"pending-byte rejection retained [%d] messages and [%d] bytes",
			len(runner.pending),
			runner.pendingBytes,
		)
	}
}

func TestShareRepairRunnerPendingQuotaDoesNotBlockOtherSender(t *testing.T) {
	runner := &shareRepairRunner{}
	for _, message := range []shareRepairMessage{
		{
			Type:          shareRepairPublicPackageMessage,
			Sender:        2,
			ContextDigest: [32]byte{0x44},
			Payload:       bytes.Repeat([]byte{0x41}, shareRepairMaximumPublicPayload),
		},
		{
			Type:          shareRepairInstalledMessage,
			Sender:        2,
			ContextDigest: [32]byte{0x44},
			Payload:       bytes.Repeat([]byte{0x42}, shareRepairMaximumSecretPayload),
		},
	} {
		if err := runner.bufferPendingMessage(message); err != nil {
			t.Fatal(err)
		}
	}
	otherSender := shareRepairMessage{
		Type:          shareRepairCompletionMessage,
		Sender:        3,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x43}, sha256.Size),
	}
	if err := runner.bufferPendingMessage(otherSender); err != nil {
		t.Fatalf("another sender was blocked by the first sender's quota: %v", err)
	}
	if err := runner.bufferPendingMessage(shareRepairMessage{
		Type:          shareRepairCompletionMessage,
		Sender:        2,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x44}, sha256.Size),
	}); err == nil || !bytes.Contains([]byte(err.Error()), []byte("for sender [2]")) {
		t.Fatalf("over-quota sender was not rejected independently: %v", err)
	}
	if len(runner.pending) != 3 || runner.pendingBytesBySender[3] != sha256.Size {
		t.Fatal("other sender's pending message was not retained")
	}
}

func TestShareRepairRunnerEnforcesTotalPendingByteBudget(t *testing.T) {
	runner := &shareRepairRunner{}
	for rawSender := 1; rawSender <= 102; rawSender++ {
		sender := group.MemberIndex(rawSender)
		for _, message := range []shareRepairMessage{
			{
				Type:          shareRepairPublicPackageMessage,
				Sender:        sender,
				ContextDigest: [32]byte{0x44},
				Payload:       bytes.Repeat([]byte{byte(sender)}, shareRepairMaximumPublicPayload),
			},
			{
				Type:          shareRepairInstalledMessage,
				Sender:        sender,
				ContextDigest: [32]byte{0x44},
				Payload:       bytes.Repeat([]byte{byte(sender)}, shareRepairMaximumSecretPayload),
			},
		} {
			if err := runner.bufferPendingMessage(message); err != nil {
				t.Fatal(err)
			}
		}
	}
	remaining := shareRepairMaximumSessionBytes - runner.pendingBytes
	if err := runner.bufferPendingMessage(shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        103,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x67}, remaining),
	}); err != nil {
		t.Fatal(err)
	}
	if runner.pendingBytes != shareRepairMaximumSessionBytes {
		t.Fatalf("runner retained [%d] total pending bytes", runner.pendingBytes)
	}
	if err := runner.bufferPendingMessage(shareRepairMessage{
		Type:          shareRepairCompletionMessage,
		Sender:        104,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x68}, sha256.Size),
	}); err == nil || !bytes.Contains([]byte(err.Error()), []byte("total pending-byte")) {
		t.Fatalf("runner did not enforce its total pending-byte budget: %v", err)
	}
}

func TestShareRepairRunnerRetainsMaximumHonestEarlyTraffic(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	stream := make(chan shareRepairMessage, 3)
	stream <- shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        2,
		ContextDigest: digest,
		Payload:       bytes.Repeat([]byte{0x41}, shareRepairMaximumPublicPayload),
	}
	stream <- shareRepairMessage{
		Type:          shareRepairDeltaMessage,
		Sender:        2,
		Recipient:     1,
		ContextDigest: digest,
		Payload:       testShareRepairCiphertext(1, 2, 1),
	}
	stream <- shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             2,
		ContextDigest:      digest,
		EphemeralPublicKey: testShareRepairPublicKey(2),
	}
	runner := &shareRepairRunner{
		member:              1,
		authorizationDigest: digest,
		transportRoster:     transportRoster,
		participants: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		stream:          stream,
		ephemeralPublic: testShareRepairPublicKey(1),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := runner.collectAnnouncements(ctx); err != nil {
		t.Fatalf("maximum honest early traffic was rejected: %v", err)
	}
	expectedBytes := shareRepairMaximumPublicPayload + shareRepairEncryptedScalarPayloadLength
	if len(runner.pending) != 2 || runner.pendingBytes != expectedBytes ||
		runner.pendingBytesBySender[2] != expectedBytes {
		t.Fatalf("honest early traffic accounting is %+v", runner)
	}
	if _, err := runner.nextMessage(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.pendingBytes != shareRepairEncryptedScalarPayloadLength || len(runner.pending) != 1 {
		t.Fatal("pending accounting did not decrement after the first message")
	}
	if _, err := runner.nextMessage(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.pending != nil || runner.pendingBytes != 0 || runner.pendingBytesBySender != nil {
		t.Fatal("pending payload references or accounting survived the final dequeue")
	}
}

func TestShareRepairRunnerFirstWinsRandomizedDeltaEncoding(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	first := testShareRepairCiphertext(1, 2, 1)
	alternate := append([]byte(nil), first...)
	alternate[len(alternate)-1] ^= 0xff
	stream := make(chan shareRepairMessage, 3)
	for _, message := range []shareRepairMessage{
		{
			Type:          shareRepairDeltaMessage,
			Sender:        2,
			Recipient:     1,
			ContextDigest: digest,
			Payload:       first,
		},
		{
			Type:          shareRepairDeltaMessage,
			Sender:        2,
			Recipient:     1,
			ContextDigest: digest,
			Payload:       alternate,
		},
		{
			Type:          shareRepairDeltaMessage,
			Sender:        1,
			Recipient:     1,
			ContextDigest: digest,
			Payload:       testShareRepairCiphertext(1, 1, 1),
		},
	} {
		stream <- message
	}
	engine := &captureShareRepairPart2Engine{testShareRepairEngine: &testShareRepairEngine{}}
	runner := &shareRepairRunner{
		member:              1,
		authorization:       authorization,
		transportRoster:     transportRoster,
		authorizationDigest: digest,
		contextWire:         fmt.Sprintf("0x%x", digest),
		helperSet: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		engine: engine,
		bus:    &manualShareRepairBus{broadcasts: make(chan shareRepairMessage, 8)},
		stream: stream,
	}
	err = runner.runHelper(context.Background(), func() {})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("stop after part2 input")) {
		t.Fatalf("helper did not reach native Part2: %v", err)
	}
	if len(engine.deltas) != 2 || engine.deltas[1].SenderIdentifier != 2 ||
		!bytes.Equal(engine.deltas[1].Payload, first) {
		t.Fatalf("helper did not retain exactly the first delta: %+v", engine.deltas)
	}
}

func TestShareRepairRunnerFirstWinsRandomizedSigmaEncoding(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	publicPackage, err := json.Marshal(testShareRepairPublicKeyPackage())
	if err != nil {
		t.Fatal(err)
	}
	first := testShareRepairCiphertext(2, 1, authorization.TargetIdentifier)
	alternate := append([]byte(nil), first...)
	alternate[len(alternate)-1] ^= 0xff
	stream := make(chan shareRepairMessage, 5)
	for _, message := range []shareRepairMessage{
		{Type: shareRepairPublicPackageMessage, Sender: 1, ContextDigest: digest, Payload: publicPackage},
		{Type: shareRepairPublicPackageMessage, Sender: 2, ContextDigest: digest, Payload: publicPackage},
		{Type: shareRepairSigmaMessage, Sender: 1, Recipient: 3, ContextDigest: digest, Payload: first},
		{Type: shareRepairSigmaMessage, Sender: 1, Recipient: 3, ContextDigest: digest, Payload: alternate},
		{Type: shareRepairSigmaMessage, Sender: 2, Recipient: 3, ContextDigest: digest, Payload: testShareRepairCiphertext(2, 2, 3)},
	} {
		stream <- message
	}
	engine := &captureShareRepairInstallEngine{testShareRepairEngine: &testShareRepairEngine{}}
	runner := &shareRepairRunner{
		member:              3,
		authorization:       authorization,
		transportRoster:     transportRoster,
		authorizationDigest: digest,
		contextWire:         fmt.Sprintf("0x%x", digest),
		helperSet: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		engine: engine,
		stream: stream,
	}
	_, err = runner.runTarget(context.Background())
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("stop after install input")) {
		t.Fatalf("target did not reach native install: %v", err)
	}
	if len(engine.sigmas) != 2 || engine.sigmas[0].HelperIdentifier != 1 ||
		!bytes.Equal(engine.sigmas[0].Payload, first) {
		t.Fatalf("target did not retain exactly the first sigma: %+v", engine.sigmas)
	}
}

func TestShareRepairRunnerStillRejectsPublicPackageEquivocation(t *testing.T) {
	authorization, _ := testShareRepairAuthorization(t)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	first, err := json.Marshal(testShareRepairPublicKeyPackage())
	if err != nil {
		t.Fatal(err)
	}
	changedPackage := testShareRepairPublicKeyPackage()
	changedPackage.VerifyingKey = "different-group-verifying-key"
	changed, err := json.Marshal(changedPackage)
	if err != nil {
		t.Fatal(err)
	}
	stream := make(chan shareRepairMessage, 2)
	stream <- shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        1,
		ContextDigest: digest,
		Payload:       first,
	}
	stream <- shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        1,
		ContextDigest: digest,
		Payload:       changed,
	}
	runner := &shareRepairRunner{
		member:              3,
		authorization:       authorization,
		authorizationDigest: digest,
		contextWire:         fmt.Sprintf("0x%x", digest),
		helperSet: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		stream: stream,
	}
	_, err = runner.runTarget(context.Background())
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("equivocated its public package")) {
		t.Fatalf("target accepted public-package equivocation: %v", err)
	}
}

func TestShareRepairRunnerDropsNonparticipantFramesBeforePendingBuffer(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	transportRoster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	stream := make(chan shareRepairMessage, shareRepairMaximumPendingMessages+2)
	for index := 0; index <= shareRepairMaximumPendingMessages; index++ {
		stream <- shareRepairMessage{
			Type:          shareRepairDeltaMessage,
			Sender:        3,
			Recipient:     1,
			ContextDigest: digest,
			Payload:       []byte{byte(index >> 8), byte(index)},
		}
	}
	stream <- shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             2,
		ContextDigest:      digest,
		EphemeralPublicKey: testShareRepairPublicKey(2),
	}
	runner := &shareRepairRunner{
		member:              1,
		authorizationDigest: digest,
		transportRoster:     transportRoster,
		participants: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		stream:          stream,
		ephemeralPublic: testShareRepairPublicKey(1),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	publicKeys, err := runner.collectAnnouncements(ctx)
	if err != nil {
		t.Fatalf("nonparticipant frames aborted rendezvous: %v", err)
	}
	if len(publicKeys) != 2 || len(runner.pending) != 0 {
		t.Fatalf(
			"nonparticipant frames reached protocol state: keys [%d], pending [%d]",
			len(publicKeys),
			len(runner.pending),
		)
	}
}
