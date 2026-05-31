package generator

import (
	"context"
	"sync"
)

type state int

const (
	working state = iota
	stopped
)

// Protocol defines the interface that allows the Scheduler to determine if the
// protocol is executing or not. This interface should be implemented by all
// important protocols of the client, such as distributed key generation or
// signing.
type Protocol interface {
	IsExecuting() bool
}

// Scheduler allows managing computationally heavy operations: stopping and
// resuming them. The client needs to generate parameters for cryptographic
// algorithms and generating these parameters requires a lot of CPU cycles.
// The generation process may starve other processes in the client when it comes
// to access to the CPU. The scheduler allows starting the parameter generation
// when no other processes such as key generation or signing are executed on the
// client. This way, the client that would normally be idle, can spend CPU
// cycles on computationally heavy operations and stop these operations when CPU
// cycles are needed elsewhere.
type Scheduler struct {
	state     state
	workers   []func(context.Context)
	stops     []context.CancelFunc
	workMutex sync.Mutex

	protocols      []Protocol
	protocolsMutex sync.Mutex
}

// RegisterProtocol adds the provided protocol to the list that will be
// inspected periodically by the Scheduler. The Scheduler checks if at least one
// of the registered protocols is running and if so, it stops the computations.
// Computations are automatically resumed once none of the protocols is
// executing. If there are no protocols registered, the scheduler continues to
// work.
func (s *Scheduler) RegisterProtocol(protocol Protocol) {
	s.protocolsMutex.Lock()
	defer s.protocolsMutex.Unlock()

	s.protocols = append(s.protocols, protocol)
}

// Compute takes the worker function and starts the computations in a separate
// goroutine if the scheduler status is "working". Otherwise, when the scheduler
// status is "stopped", the worker function is scheduled for execution later.
// The function accepts the context and is required to stop the execution if
// the context is done. The function will be called in a loop until the
// scheduler is stopped.
func (s *Scheduler) compute(workerFn func(context.Context)) {
	s.workMutex.Lock()
	defer s.workMutex.Unlock()

	s.workers = append(s.workers, workerFn)

	if s.state == working {
		s.startWorker(workerFn)
	}
}

// Stop asks all worker functions to stop their work. The context passed to
// the function is cancelled and no further calls to the worker function are
// done until the scheduler work is resumed.
func (s *Scheduler) stop() {
	s.workMutex.Lock()
	defer s.workMutex.Unlock()

	if s.state == stopped {
		return
	}

	logger.Info("stopping computations\n")
	s.state = stopped

	for _, stop := range s.stops {
		stop()
	}
	s.stops = nil
}

// Resume resumes the work of all worker functions, each in a separate
// goroutine.
func (s *Scheduler) resume() {
	s.workMutex.Lock()
	defer s.workMutex.Unlock()

	if s.state == working {
		return
	}

	logger.Info("resuming computations\n")
	s.state = working

	for _, worker := range s.workers {
		s.startWorker(worker)
	}
}

// StartWorker takes the provided worker function, creates for it an individual
// context and starts executing it in the loop until the context is done.
// This function should be executed only be the Scheduler and when the
// workMutex is locked.
func (s *Scheduler) startWorker(workerFn func(context.Context)) {
	// #nosec G118 -- cancelFn is stored in s.stops and invoked by stop().
	ctx, cancelFn := context.WithCancel(context.Background())
	s.stops = append(s.stops, cancelFn)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				workerFn(ctx)
			}
		}
	}()
}

// CheckProtocol executed a check loop over all registered protocols. If at
// least one of the protocols is currently executing, the scheduler stops all
// computations. Computations are automatically resumed once none of the
// protocols is executing. If there are no protocols registered, the scheduler
// continues to work.
func (s *Scheduler) checkProtocols() {
	s.protocolsMutex.Lock()
	defer s.protocolsMutex.Unlock()

	// No protocols and scheduler is working by default. Returning to keep it
	// working because nothing can stop the scheduler right now.
	if len(s.protocols) == 0 {
		return
	}

	atLeastOneProtocolExecuting := false

	for _, protocol := range s.protocols {
		if protocol.IsExecuting() {
			atLeastOneProtocolExecuting = true
			break
		}
	}

	if atLeastOneProtocolExecuting {
		s.stop()
	} else {
		s.resume()
	}
}
