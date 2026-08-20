package libp2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/gen/pb"
	"github.com/keep-network/keep-core/pkg/operator"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestRegisterAndFireHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	channel := &channel{}

	handlerFiredChan := make(chan struct{})
	channel.Recv(ctx, func(msg net.Message) {
		handlerFiredChan <- struct{}{}
	})

	channel.deliver(&mockNetMessage{})

	select {
	case <-handlerFiredChan:
		return
	case <-ctx.Done():
		t.Errorf("Expected handler not called")
	}
}

func TestUnregisterHandler(t *testing.T) {
	tests := map[string]struct {
		handlersRegistered   []string
		handlersUnregistered []string
		handlersFired        []string
	}{
		"unregister the first registered handler": {
			handlersRegistered:   []string{"a", "b", "c"},
			handlersUnregistered: []string{"a"},
			handlersFired:        []string{"b", "c"},
		},
		"unregister the last registered handler": {
			handlersRegistered:   []string{"a", "b", "c"},
			handlersUnregistered: []string{"c"},
			handlersFired:        []string{"a", "b"},
		},
		"unregister handler registered in the middle": {
			handlersRegistered:   []string{"a", "b", "c"},
			handlersUnregistered: []string{"b"},
			handlersFired:        []string{"a", "c"},
		},
		"unregister various handlers": {
			handlersRegistered:   []string{"a", "b", "c", "d", "e", "f", "g"},
			handlersUnregistered: []string{"a", "c", "f", "g"},
			handlersFired:        []string{"b", "d", "e"},
		},
		"unregister all handlers": {
			handlersRegistered:   []string{"a", "b", "c"},
			handlersUnregistered: []string{"a", "b", "c"},
			handlersFired:        []string{},
		},
	}

	for testName, test := range tests {
		test := test
		t.Run(testName, func(t *testing.T) {
			channel := &channel{}

			handlersFiredMutex := &sync.Mutex{}
			handlersFired := []string{}

			handlerCancellations := map[string]context.CancelFunc{}

			// Register all handlers. If the handler is called, append its
			// type to `handlersFired` slice.
			for _, handlerName := range test.handlersRegistered {
				handlerType := handlerName

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				handlerCancellations[handlerName] = cancel

				channel.Recv(ctx, func(msg net.Message) {
					handlersFiredMutex.Lock()
					handlersFired = append(handlersFired, handlerType)
					handlersFiredMutex.Unlock()
				})
			}

			// Cancel the specified handlers
			for _, handlerName := range test.handlersUnregistered {
				handlerCancellations[handlerName]()
			}

			// Deliver message, all handlers should be called
			channel.deliver(&mockNetMessage{})

			// Handlers are fired asynchronously; wait for them
			time.Sleep(500 * time.Millisecond)

			sort.Strings(handlersFired)
			if !reflect.DeepEqual(test.handlersFired, handlersFired) {
				t.Errorf(
					"Unexpected handlers fired\nExpected: %v\nActual:   %v\n",
					test.handlersFired,
					handlersFired,
				)
			}
		})
	}
}

func TestUnregisterWhenHandling(t *testing.T) {
	channel := &channel{}

	ctx, cancel := context.WithCancel(context.Background())

	receivedCount := 0
	stopAt := 90

	channel.Recv(ctx, func(msg net.Message) {
		receivedCount++

		if receivedCount == stopAt {
			cancel()
		}
	})

	go func() {
		for i := 0; i < 300; i++ {
			channel.deliver(&mockNetMessage{seqno: uint64(i)})
		}
	}()

	time.Sleep(500 * time.Millisecond)

	if receivedCount != stopAt {
		t.Fatalf("unexpected number of received messages: [%v]", receivedCount)
	}
}

func TestUnregisterWhenHandlingBlocked(t *testing.T) {
	channel := &channel{}
	receiver := make(chan interface{})

	ctx, cancel := context.WithCancel(context.Background())

	receivedCount := 0

	channel.Recv(ctx, func(msg net.Message) {
		receivedCount++
		receiver <- msg // there is no receiver, this call will block
	})

	// send a message and give some time for the handler message piping goroutine
	channel.deliver(&mockNetMessage{})
	time.Sleep(100 * time.Millisecond)

	// cancel the context and give some time for the handler lifecycle goroutine
	cancel()
	time.Sleep(100 * time.Millisecond)

	if receivedCount != 1 {
		t.Fatalf("expected just one Recv call")
	}
	if len(channel.messageHandlers) != 0 {
		t.Fatalf("expected the handler to be unregistered")
	}
}

func TestCreateTopicValidator(t *testing.T) {
	operatorPublicKeys := make([]*operator.PublicKey, 5)
	for i := range operatorPublicKeys {
		_, operatorPublicKey, _ := operator.GenerateKeyPair(DefaultCurve)
		operatorPublicKeys[i] = operatorPublicKey
	}

	authorizations := map[string]bool{
		toEncodedBytes(t, operatorPublicKeys[0]): true,
		toEncodedBytes(t, operatorPublicKeys[3]): true,
	}

	filter := func(publicKey *operator.PublicKey) bool {
		_, isAuthorized := authorizations[toEncodedBytes(t, publicKey)]
		return isAuthorized
	}

	validator := createTopicValidator(filter)

	expectedResults := []bool{true, false, false, true, false}
	for i, operatorPublicKey := range operatorPublicKeys {
		networkPublicKey, err := operatorPublicKeyToNetworkPublicKey(operatorPublicKey)
		if err != nil {
			t.Fatal(err)
		}

		authorID, _ := peer.IDFromPublicKey(networkPublicKey)
		authorIDBytes, _ := authorID.Marshal()
		message := &pubsubpb.Message{From: authorIDBytes}

		actualResult := validator(nil, peer.ID(rune(i)), &pubsub.Message{Message: message})

		if expectedResults[i] != actualResult {
			t.Errorf(
				"Unexpected result for public key of index [%v]\n"+
					"Expected: %v\nActual:   %v\n",
				i,
				expectedResults[i],
				actualResult,
			)
		}
	}
}

func toEncodedBytes(t *testing.T, publicKey *operator.PublicKey) string {
	publicKeyBytes := operator.MarshalUncompressed(publicKey)

	return hex.EncodeToString(publicKeyBytes)
}

type mockNetMessage struct {
	seqno uint64
}

func (mnm *mockNetMessage) TransportSenderID() net.TransportIdentifier {
	return &mockTransportIdentifier{"donald duck"}
}

func (mnm *mockNetMessage) Payload() interface{} {
	panic("not implemented in mock")
}

func (mnm *mockNetMessage) Type() string {
	panic("not implemented in mock")
}

func (mnm *mockNetMessage) SenderPublicKey() []byte {
	panic("not implemented in mock")
}

func (mnm *mockNetMessage) Seqno() uint64 {
	return mnm.seqno
}

type mockTransportIdentifier struct {
	transportID string
}

func (mti *mockTransportIdentifier) String() string {
	return mti.transportID
}

// TestDeliver_DropsMessageWhenHandlerFull verifies that deliver() does not
// block and silently drops the message when a handler's channel buffer is full.
func TestDeliver_DropsMessageWhenHandlerFull(t *testing.T) {
	ch := &channel{}

	// Fill the handler channel to capacity so the next send will be dropped.
	handlerCh := make(chan net.Message, messageHandlerThrottle)
	for i := 0; i < messageHandlerThrottle; i++ {
		handlerCh <- &mockNetMessage{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch.messageHandlers = []*messageHandler{
		{ctx: ctx, channel: handlerCh},
	}

	done := make(chan struct{})
	go func() {
		ch.deliver(&mockNetMessage{})
		close(done)
	}()

	select {
	case <-done:
		// deliver returned immediately -- correct
	case <-time.After(1 * time.Second):
		t.Fatal("deliver blocked when handler channel was full")
	}

	if len(handlerCh) != messageHandlerThrottle {
		t.Errorf(
			"expected handler channel to remain full (%d), got %d",
			messageHandlerThrottle,
			len(handlerCh),
		)
	}
}

// TestIncomingMessageWorker_IncrementsReceivedCounter verifies that
// incomingMessageWorker increments the message_received_total counter for
// every message dequeued, even when subsequent processing fails.
func TestIncomingMessageWorker_IncrementsReceivedCounter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	recorder := &mockMetricsRecorder{}

	ch := &channel{
		incomingMessageQueue: make(chan *pubsub.Message, incomingMessageThrottle),
		metricsRecorder:      recorder,
	}

	go ch.incomingMessageWorker(ctx)

	// A message with valid framing but no payload will fail type-lookup after
	// proto-unmarshal succeeds; the counter is incremented before processing
	// so it must still be observed. We set the inner pb.Message to avoid a
	// nil-dereference on pubsubMessage.Data.
	ch.incomingMessageQueue <- &pubsub.Message{Message: &pubsubpb.Message{}}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		recorder.mu.Lock()
		count := recorder.counters["message_received_total"]
		recorder.mu.Unlock()
		if count >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	recorder.mu.Lock()
	got := recorder.counters["message_received_total"]
	recorder.mu.Unlock()

	if got < 1 {
		t.Errorf("expected message_received_total >= 1, got %v", got)
	}
}

// TestSetMetricsRecorder_NilRecorderSkipsMonitor verifies that passing a nil
// recorder to setMetricsRecorder does not start the monitoring goroutine.
// A subsequent call with a real recorder must then start it (sync.Once is only
// consumed when recorder != nil).
func TestSetMetricsRecorder_NilRecorderSkipsMonitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := &channel{ctx: ctx}

	// First call with nil -- must not start the monitor goroutine.
	ch.setMetricsRecorder(nil)
	if ch.metricsRecorder != nil {
		t.Error("expected metricsRecorder to be nil after nil set")
	}

	// sync.Once is NOT consumed by the nil-guarded branch, so a subsequent
	// call with a real recorder must set the field.
	recorder := &mockMetricsRecorder{}
	ch.setMetricsRecorder(recorder)
	if ch.metricsRecorder == nil {
		t.Error("expected metricsRecorder to be set after non-nil set")
	}
}

type mockMetricsRecorder struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
}

func (m *mockMetricsRecorder) IncrementCounter(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = make(map[string]float64)
	}
	m.counters[name] += value
}

func (m *mockMetricsRecorder) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gauges == nil {
		m.gauges = make(map[string]float64)
	}
	m.gauges[name] = value
}

func (m *mockMetricsRecorder) RecordDuration(_ string, _ time.Duration) {}

// TestChannel_ProcessContainerMessage_UnknownType verifies that an incoming
// message whose type has no registered unmarshaler returns an error instead
// of panicking.
func TestChannel_ProcessContainerMessage_UnknownType(t *testing.T) {
	ch := &channel{
		unmarshalersByType: make(map[string]func() net.TaggedUnmarshaler),
	}

	err := ch.processContainerMessage(
		peer.ID(""),
		&pb.BroadcastNetworkMessage{Type: []byte("unknown/type")},
	)

	if err == nil {
		t.Fatal("expected error for unknown message type, got nil")
	}
	if !strings.Contains(err.Error(), "couldn't find unmarshaler") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestChannel_SetFilter_ValidatesMessages verifies that SetFilter registers a
// validator that rejects messages from unauthorized peers.
func TestChannel_SetFilter_ValidatesMessages(t *testing.T) {
	_, authorizedKey, _ := operator.GenerateKeyPair(DefaultCurve)
	_, unauthorizedKey, _ := operator.GenerateKeyPair(DefaultCurve)

	authorizedBytes := hex.EncodeToString(operator.MarshalUncompressed(authorizedKey))

	filter := func(pk *operator.PublicKey) bool {
		return hex.EncodeToString(operator.MarshalUncompressed(pk)) == authorizedBytes
	}

	mv := &mockTopicValidator{}
	ch := &channel{name: "test-channel", validator: mv}

	if err := ch.SetFilter(filter); err != nil {
		t.Fatalf("SetFilter returned unexpected error: %v", err)
	}
	if mv.registered == nil {
		t.Fatal("expected a validator to be registered")
	}

	buildMsg := func(key *operator.PublicKey) *pubsub.Message {
		netKey, err := operatorPublicKeyToNetworkPublicKey(key)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := peer.IDFromPublicKey(netKey)
		idBytes, _ := id.Marshal()
		return &pubsub.Message{Message: &pubsubpb.Message{From: idBytes}}
	}

	if !mv.registered(nil, peer.ID(""), buildMsg(authorizedKey)) {
		t.Error("expected authorized peer to pass filter")
	}
	if mv.registered(nil, peer.ID(""), buildMsg(unauthorizedKey)) {
		t.Error("expected unauthorized peer to be rejected by filter")
	}
}

// TestChannel_SetFilter_AllowsMessages verifies that a permissive filter
// allows all peers through.
func TestChannel_SetFilter_AllowsMessages(t *testing.T) {
	filter := func(_ *operator.PublicKey) bool { return true }

	mv := &mockTopicValidator{}
	ch := &channel{name: "test-channel", validator: mv}

	if err := ch.SetFilter(filter); err != nil {
		t.Fatalf("SetFilter returned unexpected error: %v", err)
	}

	_, key, _ := operator.GenerateKeyPair(DefaultCurve)
	netKey, _ := operatorPublicKeyToNetworkPublicKey(key)
	id, _ := peer.IDFromPublicKey(netKey)
	idBytes, _ := id.Marshal()
	msg := &pubsub.Message{Message: &pubsubpb.Message{From: idBytes}}

	if !mv.registered(nil, peer.ID(""), msg) {
		t.Error("expected permissive filter to allow all messages")
	}
}

// TestChannel_IncomingMessageWorker_ContextCancel verifies that
// incomingMessageWorker exits cleanly when the context is cancelled.
func TestChannel_IncomingMessageWorker_ContextCancel(t *testing.T) {
	ch := &channel{
		incomingMessageQueue: make(chan *pubsub.Message, incomingMessageThrottle),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		ch.incomingMessageWorker(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("incomingMessageWorker did not exit after context cancel")
	}
}

// TestChannel_SubscriptionWorker_ContextCancel verifies that subscriptionWorker
// exits cleanly when the context is cancelled.
func TestChannel_SubscriptionWorker_ContextCancel(t *testing.T) {
	sub := &mockSubscription{}
	ch := &channel{
		subscription:         sub,
		incomingMessageQueue: make(chan *pubsub.Message, incomingMessageThrottle),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		ch.subscriptionWorker(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("subscriptionWorker did not exit after context cancel")
	}
}

// TestChannel_MonitorQueueSizes_OverThreshold verifies that snapshotQueueSizes
// records the current incoming queue and handler queue sizes as gauges.
func TestChannel_MonitorQueueSizes_OverThreshold(t *testing.T) {
	const incomingFill = 7
	const handlerFill = 3

	incomingQueue := make(chan *pubsub.Message, incomingMessageThrottle)
	for i := 0; i < incomingFill; i++ {
		incomingQueue <- &pubsub.Message{Message: &pubsubpb.Message{}}
	}

	handlerCh := make(chan net.Message, messageHandlerThrottle)
	for i := 0; i < handlerFill; i++ {
		handlerCh <- &mockNetMessage{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := &channel{
		incomingMessageQueue: incomingQueue,
		messageHandlers:      []*messageHandler{{ctx: ctx, channel: handlerCh}},
	}

	recorder := &mockMetricsRecorder{}
	ch.snapshotQueueSizes(recorder)

	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	if got := recorder.gauges[clientinfo.MetricIncomingMessageQueueSize]; got != incomingFill {
		t.Errorf(
			"expected incoming queue gauge %v, got %v",
			incomingFill,
			got,
		)
	}

	handlerKey := fmt.Sprintf("%s_0", clientinfo.MetricMessageHandlerQueueSize)
	if got := recorder.gauges[handlerKey]; got != handlerFill {
		t.Errorf(
			"expected handler queue gauge %v, got %v",
			handlerFill,
			got,
		)
	}
}

type mockTopicValidator struct {
	registered    pubsub.Validator
	registerErr   error
	unregisterErr error
}

func (mv *mockTopicValidator) RegisterTopicValidator(
	_ string,
	val interface{},
	_ ...pubsub.ValidatorOpt,
) error {
	if v, ok := val.(pubsub.Validator); ok {
		mv.registered = v
	} else {
		return fmt.Errorf("unexpected validator type: %T", val)
	}
	return mv.registerErr
}

func (mv *mockTopicValidator) UnregisterTopicValidator(_ string) error {
	return mv.unregisterErr
}

type mockSubscription struct{}

func (ms *mockSubscription) Next(ctx context.Context) (*pubsub.Message, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (ms *mockSubscription) Cancel() {}

// --- Benchmarks ---

// BenchmarkChannelDeliver_SingleHandler measures deliver() latency with a
// single registered handler. The handler's buffer fills after messageHandlerThrottle
// calls; subsequent iterations take the non-blocking default branch. Both paths
// exercise the same mutex lock and snapshot copy overhead.
func BenchmarkChannelDeliver_SingleHandler(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := &channel{}
	ch.messageHandlers = []*messageHandler{
		{ctx: ctx, channel: make(chan net.Message, messageHandlerThrottle)},
	}
	msg := &mockNetMessage{}
	b.ResetTimer()
	for range b.N {
		ch.deliver(msg)
	}
}

// BenchmarkChannelDeliver_10Handlers measures deliver() fan-out cost across 10
// concurrent handlers -- representative of a node with multiple active protocol
// subscriptions on the same channel.
func BenchmarkChannelDeliver_10Handlers(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := &channel{}
	handlers := make([]*messageHandler, 10)
	for i := range handlers {
		handlers[i] = &messageHandler{
			ctx:     ctx,
			channel: make(chan net.Message, messageHandlerThrottle),
		}
	}
	ch.messageHandlers = handlers
	msg := &mockNetMessage{}
	b.ResetTimer()
	for range b.N {
		ch.deliver(msg)
	}
}

// BenchmarkProcessPubsubMessage measures the raw throughput of
// processPubsubMessage with an empty message. proto.Unmarshal succeeds on empty
// input; the call returns early with "couldn't find unmarshaler", giving a
// baseline for the per-message overhead before any application logic runs.
func BenchmarkProcessPubsubMessage(b *testing.B) {
	ch := &channel{
		unmarshalersByType: make(map[string]func() net.TaggedUnmarshaler),
	}
	msg := &pubsub.Message{Message: &pubsubpb.Message{}}
	b.ResetTimer()
	for range b.N {
		_ = ch.processPubsubMessage(msg)
	}
}
