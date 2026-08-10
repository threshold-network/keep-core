//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type testShareRepairEngine struct {
	mutex         sync.Mutex
	secretBuffers [][]byte
	installCalls  int
}

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

func (engine *testShareRepairEngine) rememberSecret(value []byte) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	engine.secretBuffers = append(engine.secretBuffers, value)
}

func (engine *testShareRepairEngine) ShareRepairPart1(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
) (*NativeShareRepairPart1Result, error) {
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
	for _, recipient := range authorization.HelperIdentifiers {
		secret := testShareRepairSecret(byte(helperIdentifier*10 + recipient))
		engine.rememberSecret(secret)
		result.Deltas = append(result.Deltas, &NativeShareRepairDelta{
			ContextDigest:       contextWire,
			SenderIdentifier:    helperIdentifier,
			RecipientIdentifier: recipient,
			Data:                secret,
		})
	}
	return result, nil
}

func (engine *testShareRepairEngine) ShareRepairPart2(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	deltas []*NativeShareRepairDelta,
) (*NativeShareRepairPart2Result, error) {
	if len(deltas) != len(authorization.HelperIdentifiers) {
		return nil, fmt.Errorf("wrong delta count")
	}
	for index, sender := range authorization.HelperIdentifiers {
		delta := deltas[index]
		if delta == nil || delta.SenderIdentifier != sender ||
			delta.RecipientIdentifier != helperIdentifier ||
			!bytes.Equal(delta.Data, testShareRepairSecret(byte(sender*10+helperIdentifier))) {
			return nil, fmt.Errorf("wrong delta [%d]", index)
		}
	}
	digest, _ := ComputeShareRepairAuthorizationDigest(authorization)
	secret := testShareRepairSecret(byte(100 + helperIdentifier))
	engine.rememberSecret(secret)
	contextWire := fmt.Sprintf("0x%x", digest)
	return &NativeShareRepairPart2Result{
		ContextDigest: contextWire,
		Sigma: &NativeShareRepairSigma{
			ContextDigest:    contextWire,
			HelperIdentifier: helperIdentifier,
			Data:             secret,
		},
	}, nil
}

func (engine *testShareRepairEngine) InstallRepairedShare(
	authorization *ShareRepairAuthorization,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
	sigmas []*NativeShareRepairSigma,
) (*NativeShareRepairInstallResult, error) {
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
			!bytes.Equal(sigma.Data, testShareRepairSecret(byte(100+helper))) {
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

func (bus *recordingShareRepairBus) Broadcast(message shareRepairMessage) {
	copy := message
	copy.EphemeralPublicKey = append([]byte(nil), message.EphemeralPublicKey...)
	copy.Payload = append([]byte(nil), message.Payload...)
	bus.mutex.Lock()
	bus.messages = append(bus.messages, copy)
	bus.mutex.Unlock()
	bus.shareRepairBus.Broadcast(message)
}

func TestRunShareRepairOnBusConfidentialExactSetAndInstall(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, _ := testShareRepairAuthorization(t)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	engine := &testShareRepairEngine{}
	bus := &recordingShareRepairBus{shareRepairBus: newInProcessShareRepairBus(128)}
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
	for index, secret := range engine.secretBuffers {
		if !bytes.Equal(secret, make([]byte, len(secret))) {
			engine.mutex.Unlock()
			t.Fatalf("native secret buffer [%d] was not scrubbed", index)
		}
	}
	engine.mutex.Unlock()

	bus.mutex.Lock()
	defer bus.mutex.Unlock()
	secretMessages := 0
	publicPackages := 0
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
}

func TestRunShareRepairOnBusTimesOutWithoutExactParticipantSet(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, _ := testShareRepairAuthorization(t)
	digest, _ := ComputeShareRepairAuthorizationDigest(authorization)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := runShareRepairOnBus(
		ctx,
		&testShareRepairEngine{},
		authorization,
		digest,
		[]group.MemberIndex{1},
		newInProcessShareRepairBus(16),
	)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("context deadline exceeded")) {
		t.Fatalf("expected exact-set announcement timeout, got [%v]", err)
	}
}

func TestShareRepairRunnerRejectsPendingMessageFlood(t *testing.T) {
	authorization, _ := testShareRepairAuthorization(t)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	defer keyPair.PrivateKey.Zero()

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
		participants: map[group.MemberIndex]struct{}{
			1: {},
			2: {},
		},
		stream:          stream,
		ephemeralPublic: keyPair.PublicKey.Marshal(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = runner.collectAnnouncements(ctx)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("pending-message limit")) {
		t.Fatalf("expected pending-message flood rejection, got [%v]", err)
	}
}
