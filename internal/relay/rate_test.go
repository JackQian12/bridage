package relay

import (
	"testing"
	"time"
)

// rateBucket is unexported, so this lives in package relay (white-box test).

func TestRateBucket_Unlimited(t *testing.T) {
	b := &rateBucket{}
	// rpmLimit=0 means unlimited; should always allow
	for i := 0; i < 1000; i++ {
		if !b.allow(0) {
			t.Fatalf("unlimited bucket denied at iteration %d", i)
		}
	}
}

func TestRateBucket_EnforcesLimit(t *testing.T) {
	b := &rateBucket{}
	limit := 5
	// First `limit` calls should succeed
	for i := 0; i < limit; i++ {
		if !b.allow(limit) {
			t.Fatalf("should allow call %d of %d", i+1, limit)
		}
	}
	// The (limit+1)-th call should be denied
	if b.allow(limit) {
		t.Error("bucket should deny request beyond rate limit")
	}
}

func TestRateBucket_WindowExpiry(t *testing.T) {
	b := &rateBucket{}
	limit := 3

	// Fill the bucket
	for i := 0; i < limit; i++ {
		if !b.allow(limit) {
			t.Fatalf("pre-fill failed at %d", i)
		}
	}
	// Should be blocked now
	if b.allow(limit) {
		t.Fatal("should be blocked after filling")
	}

	// Manually backdate all timestamps by 61 seconds so they expire
	b.mu.Lock()
	past := time.Now().Add(-61 * time.Second)
	for i := range b.timestamps {
		b.timestamps[i] = past
	}
	b.mu.Unlock()

	// Should be allowed again after window expires
	if !b.allow(limit) {
		t.Error("should allow after window expiry")
	}
}

func TestRateBucket_ConcurrentSafe(t *testing.T) {
	b := &rateBucket{}
	limit := 100
	done := make(chan struct{})

	allowed := make(chan bool, 500)
	for i := 0; i < 500; i++ {
		go func() {
			allowed <- b.allow(limit)
			<-done
		}()
	}
	close(done)

	count := 0
	for i := 0; i < 500; i++ {
		if <-allowed {
			count++
		}
	}
	if count > limit {
		t.Errorf("concurrent allows exceeded limit: got %d, limit %d", count, limit)
	}
}
