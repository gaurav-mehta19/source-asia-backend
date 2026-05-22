package product

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service defines the business-logic interface consumed by the controller.
type Service interface {
	Create(ctx context.Context, req *CreateRequest) (*DetailResponse, error)
	List(ctx context.Context, limit, offset int) (*ListResponse, error)
	GetByID(ctx context.Context, id string) (*DetailResponse, error)
	AddMedia(ctx context.Context, id string, req *AddMediaRequest) (*DetailResponse, error)
}

type service struct {
	repo         Repository
	maxURLs      int
	maxURLLen    int
	defaultLimit int
	maxLimit     int
}

// NewService returns a Service wired to the given repository and limits.
func NewService(repo Repository, maxURLs, maxURLLen, defaultLimit, maxLimit int) Service {
	return &service{
		repo:         repo,
		maxURLs:      maxURLs,
		maxURLLen:    maxURLLen,
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
	}
}

func (s *service) Create(ctx context.Context, req *CreateRequest) (*DetailResponse, error) {
	if err := ValidateCreateRequest(req, s.maxURLs, s.maxURLLen); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	p := &Product{
		ID:        uuid.New().String(),
		Name:      req.Name,
		SKU:       req.SKU,
		CreatedAt: now,
		UpdatedAt: now,
	}

	imgs := req.ImageURLs
	if imgs == nil {
		imgs = []string{}
	}
	vids := req.VideoURLs
	if vids == nil {
		vids = []string{}
	}
	m := &Media{ImageURLs: imgs, VideoURLs: vids}

	if err := s.repo.Create(ctx, p, m); err != nil {
		return nil, err
	}

	return &DetailResponse{
		ID:        p.ID,
		Name:      p.Name,
		SKU:       p.SKU,
		ImageURLs: imgs,
		VideoURLs: vids,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}, nil
}

func (s *service) List(ctx context.Context, limit, offset int) (*ListResponse, error) {
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}
	if offset < 0 {
		offset = 0
	}

	items, total, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return &ListResponse{
		Items: items,
		Pagination: Pagination{
			Limit:   limit,
			Offset:  offset,
			Total:   total,
			HasMore: offset+len(items) < total,
		},
	}, nil
}

func (s *service) GetByID(ctx context.Context, id string) (*DetailResponse, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) AddMedia(ctx context.Context, id string, req *AddMediaRequest) (*DetailResponse, error) {
	if err := ValidateAddMediaRequest(req, s.maxURLs, s.maxURLLen); err != nil {
		return nil, err
	}
	// Repo performs the append + read-back under a single write lock — no TOCTOU window.
	return s.repo.AddMedia(ctx, id, req.ImageURLs, req.VideoURLs)
}
