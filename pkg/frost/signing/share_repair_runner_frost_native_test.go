//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	authorization, _ := testShareRepairAuthorization(t)
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
	authorization, _ := testShareRepairAuthorization(t)
	digest, _ := ComputeShareRepairAuthorizationDigest(authorization)
	bus := &recordingShareRepairBus{shareRepairBus: newInProcessShareRepairBus(16)}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	_, err := runShareRepairOnBus(
		ctx,
		&testShareRepairEngine{},
		authorization,
		digest,
		[]group.MemberIndex{1},
		bus,
	)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("context deadline exceeded")) {
		t.Fatalf("expected exact-set announcement timeout, got [%v]", err)
	}
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
	authorization, _ := testShareRepairAuthorization(t)
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	localKeyPair, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	defer localKeyPair.PrivateKey.Zero()
	bus := &cancelTrackingShareRepairBus{
		broadcasts: make(chan *cancelTrackingShareRepairBroadcast, 8),
	}
	stream := make(chan shareRepairMessage, 4)
	runner := &shareRepairRunner{
		member:              1,
		authorization:       authorization,
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
		engine:           &testShareRepairEngine{},
		bus:              bus,
		stream:           stream,
		ephemeralPrivate: localKeyPair.PrivateKey,
		ephemeralPublic:  localKeyPair.PublicKey.Marshal(),
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
		peerKeyPair, err := ephemeral.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		stream <- shareRepairMessage{
			Type:               shareRepairAnnouncementMessage,
			Sender:             peer,
			ContextDigest:      digest,
			EphemeralPublicKey: peerKeyPair.PublicKey.Marshal(),
		}
		peerKeyPair.PrivateKey.Zero()
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
