package cache

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	cache := NewTimeCache(time.Minute)

	cache.Add("test")

	if !cache.Has("test") {
		t.Fatal("should have 'test' key")
	}
}

func TestConcurrentAdd(t *testing.T) {
	cache := NewTimeCache(time.Minute)

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func(item int) {
			cache.Add(strconv.Itoa(item))
			wg.Done()
		}(i)
	}

	wg.Wait()

	for i := 0; i < 10; i++ {
		if !cache.Has(strconv.Itoa(i)) {
			t.Fatalf("should have '%v' key", i)
		}
	}
}

func TestExpiration(t *testing.T) {
	cache := NewTimeCache(500 * time.Millisecond)
	for i := 0; i < 6; i++ {
		cache.Add(strconv.Itoa(i))
		time.Sleep(100 * time.Millisecond)
	}

	if cache.Has(strconv.Itoa(0)) {
		t.Fatal("should have dropped '0' key from the cache")
	}
}

func TestSweep(t *testing.T) {
	cache := NewTimeCache(500 * time.Millisecond)
	cache.Add("old")
	time.Sleep(100 * time.Millisecond)
	cache.Add("new")
	time.Sleep(400 * time.Millisecond)

	cache.Sweep()

	if cache.Has("old") {
		t.Fatal("should have dropped 'old' key from the cache")
	}
	if !cache.Has("new") {
		t.Fatal("should still have 'new' in the cache")
	}
}
