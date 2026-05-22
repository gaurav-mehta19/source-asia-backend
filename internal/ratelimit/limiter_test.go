package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAllow_BoundaryAcceptsThenRejects verifies the exact "5 accepted, 6th rejected" semantic.
func TestAllow_BoundaryAcceptsThenRejects(t *testing.T) {
	l := NewLimiter(5, 60)
	now := time.Now().UTC()

	for i := 1; i <= 5; i++ {
		ok, retryAfter := l.Allow("alice", now)
		if !ok {
			t.Fatalf("request %d: expected accept, got reject", i)
		}
		if retryAfter != 0 {
			t.Fatalf("request %d: accepted request must have retryAfter==0, got %d", i, retryAfter)
		}
	}

	ok, retryAfter := l.Allow("alice", now)
	if ok {
		t.Fatal("6th request: expected reject, got accept")
	}
	if retryAfter < 1 {
		t.Fatalf("rejected request must have retryAfter>=1, got %d", retryAfter)
	}
	if retryAfter > 60 {
		t.Fatalf("retryAfter must be <= window (60), got %d", retryAfter)
	}
}

// TestAllow_RetryAfterDecreasesAsWindowAges asserts retry-after reflects when the oldest slot frees.
func TestAllow_RetryAfterDecreasesAsWindowAges(t *testing.T) {
	l := NewLimiter(5, 60)
	t0 := time.Now().UTC()

	// Saturate at t0.
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow("bob", t0); !ok {
			t.Fatalf("setup: request %d unexpectedly rejected", i)
		}
	}

	// At t0+10s, oldest is 10s old → retry-after ≈ 50.
	_, retryAfter := l.Allow("bob", t0.Add(10*time.Second))
	if retryAfter < 49 || retryAfter > 51 {
		t.Fatalf("expected retryAfter ≈ 50, got %d", retryAfter)
	}

	// At t0+59s, oldest is 59s old → retry-after ≈ 1.
	_, retryAfter = l.Allow("bob", t0.Add(59*time.Second))
	if retryAfter != 1 {
		t.Fatalf("expected retryAfter == 1 (clamped), got %d", retryAfter)
	}
}

// TestAllow_WindowEvictionFreesSlot verifies a request becomes acceptable again after the window expires.
func TestAllow_WindowEvictionFreesSlot(t *testing.T) {
	l := NewLimiter(5, 60)
	t0 := time.Now().UTC()

	for i := 0; i < 5; i++ {
		l.Allow("carol", t0)
	}
	if ok, _ := l.Allow("carol", t0); ok {
		t.Fatal("saturated user should be rejected at t0")
	}
	// Skip ahead 61 seconds — all 5 timestamps are now older than the 60s window.
	if ok, _ := l.Allow("carol", t0.Add(61*time.Second)); !ok {
		t.Fatal("after window expiry, user should be accepted again")
	}
}

// TestAllow_DifferentUsersDoNotInterfere asserts per-user isolation.
func TestAllow_DifferentUsersDoNotInterfere(t *testing.T) {
	l := NewLimiter(5, 60)
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow("alice", now); !ok {
			t.Fatal("alice should not be rate limited yet")
		}
	}
	// Bob is a different user — must still be accepted.
	if ok, _ := l.Allow("bob", now); !ok {
		t.Fatal("bob (different user) must not be affected by alice's quota")
	}
}

// TestAllow_ConcurrentSameUser is the critical concurrency test required by the assignment.
// 1000 goroutines fire for the same user simultaneously; exactly 5 must be accepted.
func TestAllow_ConcurrentSameUser(t *testing.T) {
	l := NewLimiter(5, 60)
	now := time.Now().UTC()

	const goroutines = 1000
	var accepted, rejected atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if ok, _ := l.Allow("dave", now); ok {
				accepted.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if accepted.Load() != 5 {
		t.Fatalf("expected exactly 5 accepts under concurrency, got %d", accepted.Load())
	}
	if rejected.Load() != goroutines-5 {
		t.Fatalf("expected %d rejects, got %d", goroutines-5, rejected.Load())
	}
}

// TestAllow_ConcurrentDifferentUsers verifies per-user mutex sharding: many users in parallel all hit their full quota.
func TestAllow_ConcurrentDifferentUsers(t *testing.T) {
	l := NewLimiter(5, 60)
	now := time.Now().UTC()

	const users = 200
	var accepted atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for u := 0; u < users; u++ {
		userID := userKey(u)
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(uid string) {
				defer wg.Done()
				<-start
				if ok, _ := l.Allow(uid, now); ok {
					accepted.Add(1)
				}
			}(userID)
		}
	}
	close(start)
	wg.Wait()

	if accepted.Load() != users*5 {
		t.Fatalf("expected %d accepts (5 per user × %d users), got %d", users*5, users, accepted.Load())
	}
}

func userKey(i int) string {
	return "user-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	return string(buf)
}

// TestSnapshotUser_TracksAcceptedAndRejected verifies stats reflect both counters correctly.
func TestSnapshotUser_TracksAcceptedAndRejected(t *testing.T) {
	l := NewLimiter(5, 60)
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		l.Allow("eve", now)
	}
	for i := 0; i < 3; i++ {
		l.Allow("eve", now)
	}

	snap, ok := l.SnapshotUser("eve", now)
	if !ok {
		t.Fatal("expected snapshot for known user")
	}
	if snap.AcceptedInWindow != 5 {
		t.Fatalf("expected 5 accepted in window, got %d", snap.AcceptedInWindow)
	}
	if snap.RejectedTotal != 3 {
		t.Fatalf("expected 3 rejected, got %d", snap.RejectedTotal)
	}
}

// TestSnapshotUser_UnknownUserReturnsFalse asserts safe behaviour for users we've never seen.
func TestSnapshotUser_UnknownUserReturnsFalse(t *testing.T) {
	l := NewLimiter(5, 60)
	if _, ok := l.SnapshotUser("nobody", time.Now()); ok {
		t.Fatal("unknown user must return ok=false")
	}
}

// TestSnapshotUser_AcceptedInWindowExcludesExpired verifies the snapshot honours the rolling window.
func TestSnapshotUser_AcceptedInWindowExcludesExpired(t *testing.T) {
	l := NewLimiter(5, 60)
	t0 := time.Now().UTC()
	for i := 0; i < 5; i++ {
		l.Allow("frank", t0)
	}
	// View 61s later — all 5 timestamps are outside the window.
	snap, _ := l.SnapshotUser("frank", t0.Add(61*time.Second))
	if snap.AcceptedInWindow != 0 {
		t.Fatalf("expected 0 accepted in current window after expiry, got %d", snap.AcceptedInWindow)
	}
}
