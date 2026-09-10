package subscription

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUnsubscribe(t *testing.T) {
	unsubscribed := false

	subscription := NewEventSubscription(func() {
		unsubscribed = true
	})

	if unsubscribed {
		t.Fatalf("should not be unsubscribed at this point")
	}

	subscription.Unsubscribe()

	if !unsubscribed {
		t.Fatalf("should be unsubscribed at this point")
	}
}

func TestDoNotUnsubscribeTwice(t *testing.T) {
	var unsubscribeCount uint32

	subscription := NewEventSubscription(func() {
		// make the goroutine a bit longer, it increases
		// the possibility of race
		time.Sleep(10 * time.Millisecond)

		atomic.AddUint32(&unsubscribeCount, 1)
	})

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {

			subscription.Unsubscribe()
			wg.Done()
		}()
	}

	wg.Wait()

	if unsubscribeCount != 1 {
		t.Fatalf("unsubscribe handler should be called only once")
	}
}
