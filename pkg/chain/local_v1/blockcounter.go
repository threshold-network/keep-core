package local_v1

import (
	"context"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
)

type localBlockCounter struct {
	structMutex sync.Mutex
	blockHeight uint64
	waiters     map[uint64][]chan uint64
	watchers    []*watcher
}

type watcher struct {
	ctx       context.Context
	channel   chan uint64
	closeOnce sync.Once
}

var defaultBlockTime = 500 * time.Millisecond

func (lbc *localBlockCounter) WaitForBlockHeight(blockNumber uint64) error {
	waiter, err := lbc.BlockHeightWaiter(blockNumber)
	if err != nil {
		return err
	}
	<-waiter
	return nil
}

func (lbc *localBlockCounter) BlockHeightWaiter(
	blockNumber uint64,
) (<-chan uint64, error) {
	newWaiter := make(chan uint64)

	lbc.structMutex.Lock()
	defer lbc.structMutex.Unlock()

	if blockNumber <= lbc.blockHeight {
		go func() { newWaiter <- blockNumber }()
	} else {
		waiterList, exists := lbc.waiters[blockNumber]
		if !exists {
			waiterList = make([]chan uint64, 0)
		}

		lbc.waiters[blockNumber] = append(waiterList, newWaiter)
	}

	return newWaiter, nil
}

func (lbc *localBlockCounter) CurrentBlock() (uint64, error) {
	lbc.structMutex.Lock()
	defer lbc.structMutex.Unlock()
	return lbc.blockHeight, nil
}

func (lbc *localBlockCounter) WatchBlocks(ctx context.Context) <-chan uint64 {
	watcher := &watcher{
		ctx: ctx,
		// A slightly larger buffer reduces timing-sensitive block drops in tests
		// and local coordination flows while preserving non-blocking semantics.
		channel: make(chan uint64, 16),
	}

	lbc.structMutex.Lock()
	lbc.watchers = append(lbc.watchers, watcher)
	lbc.structMutex.Unlock()

	return watcher.channel
}

// count is an internal function that counts up time to simulate the generation
// of blocks.
func (lbc *localBlockCounter) count(blockTime ...time.Duration) {
	var resolvedBlockTime time.Duration
	switch len(blockTime) {
	case 1:
		resolvedBlockTime = blockTime[0]
	default:
		resolvedBlockTime = defaultBlockTime
	}

	ticker := time.NewTicker(resolvedBlockTime)

	for range ticker.C {
		lbc.structMutex.Lock()
		lbc.blockHeight++
		height := lbc.blockHeight
		waiters, exists := lbc.waiters[height]
		delete(lbc.waiters, height)
		lbc.structMutex.Unlock()

		if exists {
			for _, waiter := range waiters {
				go func(w chan uint64) { w <- height }(waiter)
			}
		}

		lbc.structMutex.Lock()
		activeWatchers := make([]*watcher, 0, len(lbc.watchers))
		for _, watcher := range lbc.watchers {
			if watcher.ctx.Err() != nil {
				watcher.closeOnce.Do(func() { close(watcher.channel) })
				continue
			}

			activeWatchers = append(activeWatchers, watcher)
		}
		lbc.watchers = activeWatchers
		watchers := make([]*watcher, len(activeWatchers))
		copy(watchers, activeWatchers)
		lbc.structMutex.Unlock()

		for _, watcher := range watchers {
			if watcher.ctx.Err() != nil {
				watcher.closeOnce.Do(func() { close(watcher.channel) })
				continue
			}

			select {
			case watcher.channel <- height: // perfect
			default: // we don't care, let's drop it
			}
		}
	}
}

// BlockCounter creates a BlockCounter that runs completely locally. It is
// designed to simply increase block height at a set time interval in the
// background.
func BlockCounter(blockTime ...time.Duration) (chain.BlockCounter, error) {
	counter := localBlockCounter{blockHeight: 0, waiters: make(map[uint64][]chan uint64)}

	go counter.count(blockTime...)

	return &counter, nil
}
