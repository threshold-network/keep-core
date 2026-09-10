package ethereum

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// BlockCounter represents a block counter.
type BlockCounter struct {
	structMutex         sync.Mutex
	latestBlockHeight   uint64
	subscriptionChannel chan block
	waiters             map[uint64][]chan uint64
	watchers            []*watcher
}

type block struct {
	Number string
}

type watcher struct {
	ctx     context.Context
	channel chan uint64
}

// WaitForBlockHeight waits for a given block height.
func (bc *BlockCounter) WaitForBlockHeight(blockNumber uint64) error {
	waiter, err := bc.BlockHeightWaiter(blockNumber)
	if err != nil {
		return err
	}
	<-waiter
	return nil
}

// BlockHeightWaiter returns a waiter for the given block.
func (bc *BlockCounter) BlockHeightWaiter(
	blockNumber uint64,
) (<-chan uint64, error) {
	newWaiter := make(chan uint64)

	bc.structMutex.Lock()
	defer bc.structMutex.Unlock()

	if blockNumber <= bc.latestBlockHeight {
		go func() { newWaiter <- blockNumber }()
	} else {
		waiterList, exists := bc.waiters[blockNumber]
		if !exists {
			waiterList = make([]chan uint64, 0)
		}

		bc.waiters[blockNumber] = append(waiterList, newWaiter)
	}

	return newWaiter, nil
}

// CurrentBlock returns the current block.
func (bc *BlockCounter) CurrentBlock() (uint64, error) {
	return bc.latestBlockHeight, nil
}

// WatchBlocks watches the blocks.
func (bc *BlockCounter) WatchBlocks(ctx context.Context) <-chan uint64 {
	watcher := &watcher{
		ctx:     ctx,
		channel: make(chan uint64),
	}

	bc.structMutex.Lock()
	bc.watchers = append(bc.watchers, watcher)
	bc.structMutex.Unlock()

	go func() {
		<-ctx.Done()

		bc.structMutex.Lock()
		for i, w := range bc.watchers {
			if w == watcher {
				bc.watchers[i] = bc.watchers[len(bc.watchers)-1]
				bc.watchers = bc.watchers[:len(bc.watchers)-1]
				break
			}
		}
		bc.structMutex.Unlock()
	}()

	return watcher.channel
}

// receiveBlocks gets each new block back from Geth and extracts the
// block height (topBlockNumber) form it. For each block height that is being
// waited on a message will be sent.
func (bc *BlockCounter) receiveBlocks() {
	for block := range bc.subscriptionChannel {
		receivedBlockHeight, err := strconv.ParseUint(block.Number, 0, 64)
		if err != nil {
			logger.Errorf("error receiving a new block: [%v]", err)
			continue
		}

		// receivedBlockHeight is the current blockchain height as just
		// received in the notification. latestBlockHeightSeen is the
		// blockchain height as observed in the previous invocation of
		// receiveBlocks().
		//
		// If we have already received notification about this block,
		// we do nothing. All handlers were already called for this block
		// height.
		if receivedBlockHeight == bc.latestBlockHeight {
			continue
		}

		// We have already seen latestBlockHeightSeen during the previous
		// execution of receiveBlocks() function and all handlers for
		// latestBlockHeightSeen were called. Now we start from the next block
		// after it and that's latestBlockHeightSeen + 1.
		for unseenBlockNumber := bc.latestBlockHeight + 1; unseenBlockNumber <= receivedBlockHeight; unseenBlockNumber++ {
			bc.structMutex.Lock()
			height := unseenBlockNumber
			bc.latestBlockHeight++
			waiters := bc.waiters[height]
			delete(bc.waiters, height)
			bc.structMutex.Unlock()

			for _, waiter := range waiters {
				go func(w chan uint64) { w <- height }(waiter)
			}

			bc.structMutex.Lock()
			watchers := make([]*watcher, len(bc.watchers))
			copy(watchers, bc.watchers)
			bc.structMutex.Unlock()

			for _, watcher := range watchers {
				if watcher.ctx.Err() != nil {
					close(watcher.channel)
					continue
				}

				select {
				case watcher.channel <- height: // perfect
				default: // we don't care, let's drop it
				}
			}
		}
	}
}

// subscribeBlocks creates a subscription to Geth to get each block.
func (bc *BlockCounter) subscribeBlocks(
	ctx context.Context,
	chainReader ChainReader,
) error {
	errorChan := make(chan error)
	newHeadChan := make(chan *Header)

	subscribe := func() {
		logger.Debugf("subscribing to new blocks")

		subscribeContext, cancel := context.WithTimeout(
			ctx,
			10*time.Second, // timeout for subscription request
		)
		defer cancel()

		subscription, err := chainReader.SubscribeNewHead(
			subscribeContext,
			newHeadChan,
		)
		if err != nil {
			logger.Warningf("could not create subscription to new blocks: [%v]", err)
			errorChan <- err
			return
		}

		for {
			select {
			case header := <-newHeadChan:
				bc.subscriptionChannel <- block{header.Number.String()}
			case err = <-subscription.Err():
				logger.Warningf("subscription to new blocks interrupted: [%v]", err)
				subscription.Unsubscribe()
				errorChan <- err
				return
			}
		}

	}

	go func() {
		for {
			go subscribe()
			<-errorChan
			time.Sleep(5 * time.Second)
		}
	}()

	lastHeader, err := chainReader.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	bc.subscriptionChannel <- block{lastHeader.Number.String()}

	return nil
}

// CreateBlockCounter creates a block counter.
func CreateBlockCounter(chainReader ChainReader) (*BlockCounter, error) {
	ctx := context.Background()

	startupHeader, err := chainReader.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to get initial block header from the chain: [%v]",
				err,
			)
	}

	blockCounter := &BlockCounter{
		latestBlockHeight:   startupHeader.Number.Uint64(),
		waiters:             make(map[uint64][]chan uint64),
		subscriptionChannel: make(chan block),
	}

	go blockCounter.receiveBlocks()
	err = blockCounter.subscribeBlocks(ctx, chainReader)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to new blocks: [%v]", err)
	}

	return blockCounter, nil
}
