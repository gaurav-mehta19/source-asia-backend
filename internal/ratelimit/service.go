package ratelimit

import (
	"context"
	"time"

	domainerrors "github.com/source-asia/backend/internal/shared/errors"
)

// Service defines the business-logic interface consumed by the controller.
type Service interface {
	ProcessRequest(ctx context.Context, userID string) (*AcceptedResponse, error)
	GetStats(ctx context.Context, userID string) (*StatsResponse, error)
}

type service struct {
	limiter *Limiter
	repo    Repository
}

// NewService wires a Limiter and Repository into the business-logic layer.
func NewService(limiter *Limiter, repo Repository) Service {
	return &service{limiter: limiter, repo: repo}
}

func (s *service) ProcessRequest(ctx context.Context, userID string) (*AcceptedResponse, error) {
	now := time.Now().UTC()
	accepted, retryAfter := s.limiter.Allow(userID, now)

	if !accepted {
		s.repo.IncrementRejected(ctx)
		return nil, domainerrors.NewRateLimited(retryAfter)
	}

	s.repo.IncrementAccepted(ctx)
	return &AcceptedResponse{
		Status:     "accepted",
		UserID:     userID,
		AcceptedAt: now,
	}, nil
}

func (s *service) GetStats(ctx context.Context, userID string) (*StatsResponse, error) {
	now := time.Now().UTC()
	globalAccepted, globalRejected := s.repo.GlobalCounts(ctx)

	resp := &StatsResponse{
		Users: make(map[string]UserStats),
		Global: GlobalStats{
			TotalAccepted: globalAccepted,
			TotalRejected: globalRejected,
		},
	}

	if userID != "" {
		snap, ok := s.limiter.SnapshotUser(userID, now)
		if ok {
			resp.Users[userID] = UserStats{
				AcceptedInCurrentWindow: snap.AcceptedInWindow,
				RejectedTotal:           snap.RejectedTotal,
				WindowStartedAt:         snap.WindowStartedAt,
			}
		}
		return resp, nil
	}

	for id, snap := range s.limiter.Snapshot(now) {
		resp.Users[id] = UserStats{
			AcceptedInCurrentWindow: snap.AcceptedInWindow,
			RejectedTotal:           snap.RejectedTotal,
			WindowStartedAt:         snap.WindowStartedAt,
		}
	}
	return resp, nil
}
