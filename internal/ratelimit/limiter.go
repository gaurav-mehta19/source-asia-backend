package ratelimit

import (
	"sync"
	"time"
)

// userState holds the per-user rolling window and cumulative reject counter.
// A dedicated mutex per user means concurrent requests for different users
// never contend on the same lock, and concurrent requests for the same user
// are serialised only at the per-user level — not globally.
type userState struct {
	mu            sync.Mutex
	windowTimes   []time.Time // timestamps of accepted requests within the current window
	rejectedTotal int
}

// Limiter is a sliding-window rate limiter.
// Concurrency model:
//   - globalMu protects the users map itself (add/lookup entries).
//   - Each *userState carries its own mutex for per-user check-and-record atomicity.
//   - A request acquires globalMu (read or write) to get the *userState pointer,
//     then releases globalMu before acquiring the per-user mutex. This two-phase
//     approach eliminates global lock contention during the actual window check.
type Limiter struct {
	maxRequests    int
	windowDuration time.Duration

	globalMu sync.Mutex
	users    map[string]*userState
}

// NewLimiter creates a Limiter that allows at most maxRequests per windowDuration per user.
func NewLimiter(maxRequests int, windowSeconds int) *Limiter {
	return &Limiter{
		maxRequests:    maxRequests,
		windowDuration: time.Duration(windowSeconds) * time.Second,
		users:          make(map[string]*userState),
	}
}

// getOrCreate returns the existing *userState for userID or creates one atomically.
func (l *Limiter) getOrCreate(userID string) *userState {
	l.globalMu.Lock()
	defer l.globalMu.Unlock()
	s, ok := l.users[userID]
	if !ok {
		s = &userState{}
		l.users[userID] = s
	}
	return s
}

// Allow performs an atomic check-and-record:
//  1. Drops timestamps outside the rolling window.
//  2. If count < maxRequests, records the current timestamp and returns true.
//  3. Otherwise increments the reject counter and returns false.
//
// Returns (accepted bool, retryAfterSeconds int).
// The entire operation is held under the per-user mutex so no read-then-write race is possible.
func (l *Limiter) Allow(userID string, now time.Time) (bool, int) {
	state := l.getOrCreate(userID)

	state.mu.Lock()
	defer state.mu.Unlock()

	cutoff := now.Add(-l.windowDuration)

	// Evict timestamps older than the rolling window (sliding-window log).
	valid := state.windowTimes[:0]
	for _, t := range state.windowTimes {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	state.windowTimes = valid

	if len(state.windowTimes) < l.maxRequests {
		state.windowTimes = append(state.windowTimes, now)
		return true, 0
	}

	state.rejectedTotal++

	// Oldest accepted request determines when a slot opens.
	oldest := state.windowTimes[0]
	retryAfter := int(l.windowDuration.Seconds() - now.Sub(oldest).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	return false, retryAfter
}

// Snapshot returns a point-in-time view of all user states for the stats endpoint.
// It acquires per-user locks one at a time — acceptable because stats is a read-heavy,
// low-frequency endpoint and we never need a globally consistent snapshot.
func (l *Limiter) Snapshot(now time.Time) map[string]LimiterUserSnapshot {
	l.globalMu.Lock()
	// Copy the map of pointers while holding the global lock.
	usersCopy := make(map[string]*userState, len(l.users))
	for id, s := range l.users {
		usersCopy[id] = s
	}
	l.globalMu.Unlock()

	result := make(map[string]LimiterUserSnapshot, len(usersCopy))
	cutoff := now.Add(-l.windowDuration)

	for id, s := range usersCopy {
		s.mu.Lock()
		count := 0
		var windowStart time.Time
		for _, t := range s.windowTimes {
			if t.After(cutoff) {
				if windowStart.IsZero() {
					windowStart = t
				}
				count++
			}
		}
		rejected := s.rejectedTotal
		s.mu.Unlock()

		if windowStart.IsZero() {
			windowStart = now
		}
		result[id] = LimiterUserSnapshot{
			AcceptedInWindow: count,
			RejectedTotal:    rejected,
			WindowStartedAt:  windowStart,
		}
	}
	return result
}

// SnapshotUser returns a snapshot for a single user (or zero value if unknown).
func (l *Limiter) SnapshotUser(userID string, now time.Time) (LimiterUserSnapshot, bool) {
	l.globalMu.Lock()
	s, ok := l.users[userID]
	l.globalMu.Unlock()
	if !ok {
		return LimiterUserSnapshot{}, false
	}

	cutoff := now.Add(-l.windowDuration)
	s.mu.Lock()
	count := 0
	var windowStart time.Time
	for _, t := range s.windowTimes {
		if t.After(cutoff) {
			if windowStart.IsZero() {
				windowStart = t
			}
			count++
		}
	}
	rejected := s.rejectedTotal
	s.mu.Unlock()

	if windowStart.IsZero() {
		windowStart = now
	}
	return LimiterUserSnapshot{
		AcceptedInWindow: count,
		RejectedTotal:    rejected,
		WindowStartedAt:  windowStart,
	}, true
}

// LimiterUserSnapshot is a point-in-time read of a single user's state.
type LimiterUserSnapshot struct {
	AcceptedInWindow int
	RejectedTotal    int
	WindowStartedAt  time.Time
}
