package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
)

var (
	ErrInvalidID = errors.New("id is required")
	ErrNotFound  = errors.New("resource not found")
)

type CRUDService[T any] interface {
	Create(ctx context.Context, item T) (T, error)
	Update(ctx context.Context, id string, item T) (T, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (T, error)
	List(ctx context.Context, query models.PageQuery) ([]T, error)
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[T], error)
	SetStatus(ctx context.Context, id string, enabled bool) (models.ResourceStatus, error)
}

type BeforeSaveFunc[T any] func(*T)

type Service[T any] struct {
	repo       repositories.CRUDRepository[T]
	beforeSave BeforeSaveFunc[T]
}

func NewService[T any](repo repositories.CRUDRepository[T], beforeSave BeforeSaveFunc[T]) *Service[T] {
	return &Service[T]{repo: repo, beforeSave: beforeSave}
}

func (s *Service[T]) Create(ctx context.Context, item T) (T, error) {
	if s.beforeSave != nil {
		s.beforeSave(&item)
	}
	return s.repo.Create(ctx, item)
}

func (s *Service[T]) Update(ctx context.Context, id string, item T) (T, error) {
	if strings.TrimSpace(id) == "" {
		var zero T
		return zero, ErrInvalidID
	}
	if s.beforeSave != nil {
		s.beforeSave(&item)
	}
	out, err := s.repo.Update(ctx, id, item)
	return out, mapRepoError(err)
}

func (s *Service[T]) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidID
	}
	return mapRepoError(s.repo.Delete(ctx, id))
}

func (s *Service[T]) GetByID(ctx context.Context, id string) (T, error) {
	if strings.TrimSpace(id) == "" {
		var zero T
		return zero, ErrInvalidID
	}
	out, err := s.repo.GetByID(ctx, id)
	return out, mapRepoError(err)
}

func (s *Service[T]) List(ctx context.Context, query models.PageQuery) ([]T, error) {
	items, err := s.repo.List(ctx, query)
	return items, mapRepoError(err)
}

func (s *Service[T]) Page(ctx context.Context, query models.PageQuery) (models.PageResult[T], error) {
	out, err := s.repo.Page(ctx, query)
	return out, mapRepoError(err)
}

func (s *Service[T]) SetStatus(ctx context.Context, id string, enabled bool) (models.ResourceStatus, error) {
	if strings.TrimSpace(id) == "" {
		return models.ResourceStatus{}, ErrInvalidID
	}
	out, err := s.repo.SetStatus(ctx, id, enabled)
	return out, mapRepoError(err)
}

func mapRepoError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repositories.ErrInvalidID):
		return ErrInvalidID
	case errors.Is(err, repositories.ErrNotFound):
		return ErrNotFound
	default:
		return err
	}
}
