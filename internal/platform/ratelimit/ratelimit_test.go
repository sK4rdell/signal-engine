package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestLimiter_AllowsUpToLimitThenBlocks(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("ip"); !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	ok, retry := l.Allow("ip")
	if ok {
		t.Fatal("4th request should be blocked")
	}
	if retry != time.Minute {
		t.Errorf("retryAfter = %v, want 1m", retry)
	}

	if ok, _ := l.Allow("other"); !ok {
		t.Error("other keys are independent")
	}

	now = now.Add(time.Minute)
	if ok, _ := l.Allow("ip"); !ok {
		t.Error("window should have reset")
	}
}

func TestLimiter_GarbageCollectsExpiredBuckets(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New(1, time.Second)
	l.now = func() time.Time { return now }

	l.Allow("a")
	l.Allow("b")
	now = now.Add(2 * time.Second)
	l.Allow("c")
	if len(l.buckets) != 1 {
		t.Errorf("buckets = %d, want 1 after gc", len(l.buckets))
	}
}

func TestLimiter_IsSafeForConcurrentUse(t *testing.T) {
	l := New(50, time.Minute)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.Allow("k"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Errorf("allowed = %d, want exactly 50", allowed)
	}
}
