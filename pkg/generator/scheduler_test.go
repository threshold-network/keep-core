package generator

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
)

var one = big.NewInt(1)

type testWorker struct {
	mu       sync.Mutex
	number   *big.Int
	iterated chan struct{}
	lastCtx  context.Context
}

func newTestWorker() *testWorker {
	return &testWorker{
		number:   big.NewInt(0),
		iterated: make(chan struct{}, 1),
	}
}

func (tw *testWorker) workerFunc() func(context.Context) {
	return func(ctx context.Context) {
		tw.mu.Lock()
		tw.lastCtx = ctx
		if ctx.Err() != nil {
			tw.mu.Unlock()
			return
		}

		tw.number.Add(tw.number, one)
		tw.mu.Unlock()

		select {
		case tw.iterated <- struct{}{}:
		default:
		}
	}
}

func (tw *testWorker) value() *big.Int {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return new(big.Int).Set(tw.number)
}

func (tw *testWorker) waitForStop(t *testing.T, timeout time.Duration) {
	t.Helper()
	tw.mu.Lock()
	ctx := tw.lastCtx
	tw.mu.Unlock()

	if ctx == nil {
		t.Fatal("worker has not started yet")
	}

	select {
	case <-ctx.Done():
	case <-time.After(timeout):
		t.Fatal("timed out waiting for worker to stop")
	}

	// Take the lock so no in-flight execution of workerFunc is inside its
	// increment critical section when the value is read next; its channel
	// send may still be pending.
	tw.mu.Lock()
	tw.mu.Unlock()
}

func (tw *testWorker) waitForValueChange(t *testing.T, initial *big.Int, timeout time.Duration) *big.Int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current := tw.value()
		if current.Cmp(initial) != 0 {
			return current
		}
		select {
		case <-tw.iterated:
		case <-time.After(10 * time.Millisecond):
		}
	}
	current := tw.value()
	if current.Cmp(initial) != 0 {
		return current
	}
	t.Fatalf("timed out waiting for worker value to change from %v", initial)
	return nil
}

func (tw *testWorker) waitForNonZero(t *testing.T, timeout time.Duration) *big.Int {
	t.Helper()
	return tw.waitForValueChange(t, big.NewInt(0), timeout)
}

// assertNoValueChange fails if the worker performs any further computation
// within the given window.
//
// Proving that nothing happens needs an observation window; there is no event to
// wait for. The window is short because a running worker increments on every
// scheduler iteration, so it is caught almost immediately.
//
// The value, not the tw.iterated signal, is the source of truth: workerFunc
// increments the counter and sends on the channel as two separate steps, so a
// worker that incremented before the stop can still deliver its signal after
// it. The signal is only used to re-check early; a late delivery from a
// completed increment must not count as execution.
func (tw *testWorker) assertNoValueChange(t *testing.T, window time.Duration) {
	t.Helper()

	initial := tw.value()
	deadline := time.After(window)

	for {
		select {
		case <-tw.iterated:
			if current := tw.value(); current.Cmp(initial) != 0 {
				t.Fatalf(
					"worker executed after stop: initial value %v, current value %v",
					initial,
					current,
				)
			}
		case <-deadline:
			testutils.AssertBigIntsEqual(
				t,
				"computation result after stop signal",
				initial,
				tw.value(),
			)
			return
		}
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s", msg)
	}
}

// TestComputeStop tests the situation when two new worker functions are added
// to a scheduler in a working state. The test ensures the worker functions
// starts doing their work. Then, the scheduler is stopped and the test ensures
// the worker functions are stopped.
func TestComputeStop(t *testing.T) {
	scheduler := new(Scheduler)

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	// ensure computations started
	val1 := tw1.waitForNonZero(t, 1*time.Second)
	val2 := tw2.waitForNonZero(t, 1*time.Second)
	testutils.AssertBigIntNonZero(t, "computation result", val1)
	testutils.AssertBigIntNonZero(t, "computation result", val2)

	// send the stop signal and wait for computations to stop
	scheduler.stop()
	tw1.waitForStop(t, 1*time.Second)
	tw2.waitForStop(t, 1*time.Second)

	// ensure computations stopped
	tw1.assertNoValueChange(t, 100*time.Millisecond)
	tw2.assertNoValueChange(t, 100*time.Millisecond)
}

// TestComputeStopContext covers the same situation as TestComputeStop except
// that it ensures if the context passed to the function is cancelled when the
// work is stopped.
func TestComputeStopContext(t *testing.T) {
	scheduler := new(Scheduler)

	started1 := make(chan struct{})
	started2 := make(chan struct{})
	cancelled1 := make(chan struct{})
	cancelled2 := make(chan struct{})

	var onceStarted1, onceStarted2 sync.Once
	var onceCancelled1, onceCancelled2 sync.Once

	scheduler.compute(func(ctx context.Context) {
		// this simulates a long-running task
		onceStarted1.Do(func() {
			close(started1)
		})
		<-ctx.Done()
		onceCancelled1.Do(func() {
			close(cancelled1)
		})
	})
	scheduler.compute(func(ctx context.Context) {
		// this simulates a long-running task
		onceStarted2.Do(func() {
			close(started2)
		})
		<-ctx.Done()
		onceCancelled2.Do(func() {
			close(cancelled2)
		})
	})

	// wait for workers to start and reach the long-running task
	waitForSignal(t, started1, 1*time.Second, "worker 1 start")
	waitForSignal(t, started2, 1*time.Second, "worker 2 start")

	// send the stop signal
	scheduler.stop()

	// ensure context got cancelled
	waitForSignal(t, cancelled1, 1*time.Second, "worker 1 context cancellation")
	waitForSignal(t, cancelled2, 1*time.Second, "worker 2 context cancellation")
}

// TestComputeStopResume tests the situation when two new worker functions are
// added to a scheduler in a working state. Then, the scheduler is stopped and
// after some time its work is resumed. The test ensures the worker functions
// resume their work.
func TestComputeStopResume(t *testing.T) {
	scheduler := new(Scheduler)
	defer scheduler.stop()

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	// ensure computations started
	tw1.waitForNonZero(t, 1*time.Second)
	tw2.waitForNonZero(t, 1*time.Second)

	// send the stop signal and wait for computations to stop
	scheduler.stop()
	tw1.waitForStop(t, 1*time.Second)
	tw2.waitForStop(t, 1*time.Second)

	// at this point, all computations should be stopped, capture the current
	// result
	intermediateResult1 := tw1.value()
	intermediateResult2 := tw2.value()

	// send the resume signal and ensure computations have been resumed
	scheduler.resume()
	tw1.waitForValueChange(t, intermediateResult1, 1*time.Second)
	tw2.waitForValueChange(t, intermediateResult2, 1*time.Second)

	testutils.AssertBigIntsNotEqual(
		t,
		"computation results after resume signal",
		intermediateResult1,
		tw1.value(),
	)
	testutils.AssertBigIntsNotEqual(
		t,
		"computation results after resume signal",
		intermediateResult2,
		tw2.value(),
	)
}

// TestComputeStopResumeStop tests the situation when two new worker functions
// are added to a scheduler in a working state. Then, the scheduler is stopped,
// resumed, and stopped again. The test ensures the worker functions are stopped
// at the end of the cycle.
func TestComputeStopResumeStop(t *testing.T) {
	scheduler := new(Scheduler)

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	// ensure computations started
	tw1.waitForNonZero(t, 1*time.Second)
	tw2.waitForNonZero(t, 1*time.Second)

	scheduler.stop()
	tw1.waitForStop(t, 1*time.Second)
	tw2.waitForStop(t, 1*time.Second)

	scheduler.resume()
	tw1.waitForValueChange(t, tw1.value(), 1*time.Second)
	tw2.waitForValueChange(t, tw2.value(), 1*time.Second)

	scheduler.stop()
	tw1.waitForStop(t, 1*time.Second)
	tw2.waitForStop(t, 1*time.Second)

	// ensure computations stopped
	tw1.assertNoValueChange(t, 100*time.Millisecond)
	tw2.assertNoValueChange(t, 100*time.Millisecond)
}

// TestStopComputeResume tests the situation when two new worker functions are
// added to a stopped scheduler. Then, the scheduler work is resumed. The test
// ensures the worker functions are not working before the resume and that they
// are working after the resume.
func TestStopComputeResume(t *testing.T) {
	scheduler := new(Scheduler)
	defer scheduler.stop()

	scheduler.stop()

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	// assert computations have not started - the scheduler is stopped
	testutils.AssertBigIntsEqual(t, "computation result", big.NewInt(0), tw1.value())
	testutils.AssertBigIntsEqual(t, "computation result", big.NewInt(0), tw2.value())

	scheduler.resume()

	// ensure computations started
	val1 := tw1.waitForNonZero(t, 1*time.Second)
	val2 := tw2.waitForNonZero(t, 1*time.Second)
	testutils.AssertBigIntNonZero(t, "computation result", val1)
	testutils.AssertBigIntNonZero(t, "computation result", val2)
}

// TestCheckProtocols_NoProtocols ensures the execution of checkProtocols
// does not stop the scheduler if there are no protocols registered.
func TestCheckProtocols_NoProtocols(t *testing.T) {
	scheduler := new(Scheduler)
	defer scheduler.stop()

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	// ensure computations started
	tw1.waitForNonZero(t, 1*time.Second)
	tw2.waitForNonZero(t, 1*time.Second)

	scheduler.checkProtocols()

	// there are no protocols executed, nothing can stop the scheduler;
	// ensure the computations are performed
	intermediateResult1 := tw1.value()
	intermediateResult2 := tw2.value()

	tw1.waitForValueChange(t, intermediateResult1, 1*time.Second)
	tw2.waitForValueChange(t, intermediateResult2, 1*time.Second)

	testutils.AssertBigIntsNotEqual(
		t,
		"computation result after stop signal",
		intermediateResult1,
		tw1.value(),
	)
	testutils.AssertBigIntsNotEqual(
		t,
		"computation result after stop signal",
		intermediateResult2,
		tw2.value(),
	)
}

// TestCheckProtocols_ProtocolNotExecuting ensures the execution of checkProtocols
// does not stop the scheduler if there are protocols registered but they are
// not executing.
func TestCheckProtocols_ProtocolNotExecuting(t *testing.T) {
	scheduler := new(Scheduler)
	defer scheduler.stop()

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	protocol1 := &mockProtocol{}
	protocol2 := &mockProtocol{}
	scheduler.RegisterProtocol(protocol1)
	scheduler.RegisterProtocol(protocol2)

	// ensure computations started
	tw1.waitForNonZero(t, 1*time.Second)
	tw2.waitForNonZero(t, 1*time.Second)

	scheduler.checkProtocols()

	// there are two protocols but they are not executing;
	// ensure the computations are performed
	intermediateResult1 := tw1.value()
	intermediateResult2 := tw2.value()

	tw1.waitForValueChange(t, intermediateResult1, 1*time.Second)
	tw2.waitForValueChange(t, intermediateResult2, 1*time.Second)

	testutils.AssertBigIntsNotEqual(
		t,
		"computation result after stop signal",
		intermediateResult1,
		tw1.value(),
	)
	testutils.AssertBigIntsNotEqual(
		t,
		"computation result after stop signal",
		intermediateResult2,
		tw2.value(),
	)
}

// TestCheckProtocols_ProtocolExecuting ensures the execution of checkProtocols
// does stop the scheduler if at least of the registered protocols is executing.
func TestCheckProtocols_ProtocolExecuting(t *testing.T) {
	scheduler := new(Scheduler)
	defer scheduler.stop()

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	protocol1 := &mockProtocol{}
	protocol2 := &mockProtocol{}
	scheduler.RegisterProtocol(protocol1)
	scheduler.RegisterProtocol(protocol2)

	// ensure computations started
	tw1.waitForNonZero(t, 1*time.Second)
	tw2.waitForNonZero(t, 1*time.Second)

	protocol2.isExecuting = true
	scheduler.checkProtocols()

	// wait for computations to stop
	tw1.waitForStop(t, 1*time.Second)
	tw2.waitForStop(t, 1*time.Second)

	// there are two protocols and the second one is executing;
	// ensure the computations are stopped
	tw1.assertNoValueChange(t, 100*time.Millisecond)
	tw2.assertNoValueChange(t, 100*time.Millisecond)
}

// TestCheckProtocols_ProtocolFinishedExecution ensures the execution of
// checkProtocols resumes the work of the scheduler if the protocol that was
// previously executing finished the work.
func TestCheckProtocols_ProtocolFinishedExecution(t *testing.T) {
	scheduler := new(Scheduler)
	defer scheduler.stop()

	tw1 := newTestWorker()
	tw2 := newTestWorker()

	scheduler.compute(tw1.workerFunc())
	scheduler.compute(tw2.workerFunc())

	protocol1 := &mockProtocol{}
	protocol2 := &mockProtocol{}
	scheduler.RegisterProtocol(protocol1)
	scheduler.RegisterProtocol(protocol2)

	// ensure computations started
	tw1.waitForNonZero(t, 1*time.Second)
	tw2.waitForNonZero(t, 1*time.Second)

	protocol2.isExecuting = true
	scheduler.checkProtocols()

	// wait for computations to stop
	tw1.waitForStop(t, 1*time.Second)
	tw2.waitForStop(t, 1*time.Second)

	protocol2.isExecuting = false
	scheduler.checkProtocols()

	// there are two protocols, the second one was executing, but it has
	// finished; ensure the computations are resumed
	intermediateResult1 := tw1.value()
	intermediateResult2 := tw2.value()

	tw1.waitForValueChange(t, intermediateResult1, 1*time.Second)
	tw2.waitForValueChange(t, intermediateResult2, 1*time.Second)

	testutils.AssertBigIntsNotEqual(
		t,
		"computation result after stop signal",
		intermediateResult1,
		tw1.value(),
	)
	testutils.AssertBigIntsNotEqual(
		t,
		"computation result after stop signal",
		intermediateResult2,
		tw2.value(),
	)
}

type mockProtocol struct {
	isExecuting bool
}

func (mp *mockProtocol) IsExecuting() bool {
	return mp.isExecuting
}
