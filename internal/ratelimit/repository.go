package ratelimit

import (
	"context"
	"sync/atomic"
)

// Repository defines the persistence interface for rate-limit counters.
type Repository interface {
	IncrementAccepted(ctx context.Context)
	IncrementRejected(ctx context.Context)
	GlobalCounts(ctx context.Context) (accepted, rejected int)
}

// memRepository is an in-memory Repository backed by atomic integers.
type memRepository struct {
	totalAccepted atomic.Int64
	totalRejected atomic.Int64
}

func newMemRepository() *memRepository {
	return &memRepository{}
}

func (r *memRepository) IncrementAccepted(_ context.Context) {
	r.totalAccepted.Add(1)
}

func (r *memRepository) IncrementRejected(_ context.Context) {
	r.totalRejected.Add(1)
}

func (r *memRepository) GlobalCounts(_ context.Context) (int, int) {
	return int(r.totalAccepted.Load()), int(r.totalRejected.Load())
}
