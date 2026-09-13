package ethereum

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitForNewMinedBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	blockCounter := &BlockCounter{
		latestBlockHeight:   uint64(1),
		waiters:             make(map[uint64][]chan uint64),
		subscriptionChannel: make(chan block),
	}

	block2Waiter, err := blockCounter.BlockHeightWaiter(2)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		blockCounter.subscriptionChannel <- block{Number: "1"}
		blockCounter.subscriptionChannel <- block{Number: "2"}
	}()

	go blockCounter.receiveBlocks()

	select {
	case block := <-block2Waiter:
		if block != 2 {
			t.Fatalf(
				"unexpected block number\nexpected: [2]\nactual:   [%v]\n",
				block,
			)
		}

	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestExecuteBlockHandlerOnlyOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	blockCounter := &BlockCounter{
		latestBlockHeight:   uint64(1),
		waiters:             make(map[uint64][]chan uint64),
		subscriptionChannel: make(chan block),
	}

	block2Waiter, err := blockCounter.BlockHeightWaiter(2)
	if err != nil {
		t.Fatal(err)
	}

	block3Waiter, err := blockCounter.BlockHeightWaiter(3)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		blockCounter.subscriptionChannel <- block{Number: "1"}
		blockCounter.subscriptionChannel <- block{Number: "2"}
		blockCounter.subscriptionChannel <- block{Number: "2"}
		blockCounter.subscriptionChannel <- block{Number: "3"}
		blockCounter.subscriptionChannel <- block{Number: "4"}
	}()

	go blockCounter.receiveBlocks()

	var waiter2CallCounter uint64
	var waiter3CallCounter uint64

	for {
		select {
		case <-block2Waiter:
			atomic.AddUint64(&waiter2CallCounter, 1)

		case <-block3Waiter:
			atomic.AddUint64(&waiter3CallCounter, 1)

		case <-ctx.Done():
			if waiter2CallCounter != 1 {
				t.Errorf(
					"handler for block 2 should be called only once; was [%v]",
					waiter2CallCounter,
				)
			}
			if waiter3CallCounter != 1 {
				t.Errorf(
					"handler for block 3 should be called only once; was [%v]",
					waiter3CallCounter,
				)
			}

			return
		}
	}
}

func TestWatchBlocks(t *testing.T) {
	blockCounter := &BlockCounter{
		latestBlockHeight:   uint64(1),
		waiters:             make(map[uint64][]chan uint64),
		subscriptionChannel: make(chan block),
	}
	go blockCounter.receiveBlocks()

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	watcher1 := blockCounter.WatchBlocks(ctx1)
	watcher2 := blockCounter.WatchBlocks(ctx2)

	watcher1ReceivedCount := 0
	watcher2ReceivedCount := 0
	go func() {
		for range watcher1 {
			watcher1ReceivedCount++
		}
	}()
	go func() {
		for range watcher2 {
			watcher2ReceivedCount++
		}
	}()
	// give some time for watcher goroutine to initialize
	time.Sleep(50 * time.Millisecond)

	blockCounter.subscriptionChannel <- block{Number: "2"}
	time.Sleep(50 * time.Millisecond)
	cancel1()
	blockCounter.subscriptionChannel <- block{Number: "3"}
	time.Sleep(50 * time.Millisecond)
	cancel2()
	blockCounter.subscriptionChannel <- block{Number: "4"}
	time.Sleep(50 * time.Millisecond)

	if watcher1ReceivedCount != 1 {
		t.Errorf("watcher 1 should receive [1] block, has [%v]", watcher1ReceivedCount)
	}
	if watcher2ReceivedCount != 2 {
		t.Errorf("watcher 2 should receive [2] block, has [%v]", watcher2ReceivedCount)
	}
}

func TestWatchBlocksIsNonBlocking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	blockCounter := &BlockCounter{
		latestBlockHeight:   uint64(1),
		waiters:             make(map[uint64][]chan uint64),
		subscriptionChannel: make(chan block),
	}
	go blockCounter.receiveBlocks()

	_ = blockCounter.WatchBlocks(ctx)        // does not read blocks
	watcher := blockCounter.WatchBlocks(ctx) // does read blocks

	var receivedCount uint64
	go func() {
		for range watcher {
			receivedCount++
		}
	}()
	// give some time for watcher goroutine to initialize
	time.Sleep(50 * time.Millisecond)

	blockCounter.subscriptionChannel <- block{Number: "2"}
	time.Sleep(10 * time.Millisecond)
	blockCounter.subscriptionChannel <- block{Number: "3"}
	time.Sleep(10 * time.Millisecond)

	if receivedCount != 2 {
		t.Fatalf("watcher should receive [2] blocks, has [%v]", receivedCount)
	}
}
