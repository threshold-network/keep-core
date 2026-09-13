package cache

import (
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

type valueType struct {
	field int
}

func TestGenericTimeCache_Add(t *testing.T) {
	cache := NewGenericTimeCache[*valueType](time.Minute)

	cache.Add("test", &valueType{10})

	value, ok := cache.Get("test")
	if !ok {
		t.Fatal("should have 'test' key")
	}

	expectedValue := &valueType{10}
	if !reflect.DeepEqual(expectedValue, value) {
		t.Errorf(
			"unexpected value: \n"+
				"exptected: %v\n"+
				"actual:    %v",
			expectedValue,
			value,
		)
	}
}

func TestGenericTimeCache_ConcurrentAdd(t *testing.T) {
	cache := NewGenericTimeCache[*valueType](time.Minute)

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func(item int) {
			cache.Add(strconv.Itoa(item), &valueType{item})
			wg.Done()
		}(i)
	}

	wg.Wait()

	for i := 0; i < 10; i++ {
		value, ok := cache.Get(strconv.Itoa(i))
		if !ok {
			t.Fatalf("should have '%v' key", i)
		}

		expectedValue := &valueType{i}
		if !reflect.DeepEqual(expectedValue, value) {
			t.Errorf(
				"unexpected value: \n"+
					"exptected: %v\n"+
					"actual:    %v",
				expectedValue,
				value,
			)
		}
	}
}

func TestGenericTimeCache_Expiration(t *testing.T) {
	cache := NewGenericTimeCache[*valueType](500 * time.Millisecond)
	for i := 0; i < 6; i++ {
		cache.Add(strconv.Itoa(i), &valueType{i})
		time.Sleep(100 * time.Millisecond)
	}

	if _, ok := cache.Get(strconv.Itoa(0)); ok {
		t.Fatal("should have dropped '0' key from the cache")
	}
}

func TestGenericTimeCache_Sweep(t *testing.T) {
	cache := NewGenericTimeCache[*valueType](500 * time.Millisecond)
	cache.Add("old", &valueType{10})
	time.Sleep(100 * time.Millisecond)
	cache.Add("new", &valueType{20})
	time.Sleep(400 * time.Millisecond)

	cache.Sweep()

	if _, ok := cache.Get("old"); ok {
		t.Fatal("should have dropped 'old' key from the cache")
	}
	if _, ok := cache.Get("new"); !ok {
		t.Fatal("should still have 'new' in the cache")
	}
}
