package repositories

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

var (
	ErrNotFound    = errors.New("resource not found")
	ErrInvalidID   = errors.New("resource id is required")
	validOrderName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

type CRUDRepository[T any] interface {
	Create(ctx context.Context, item T) (T, error)
	Update(ctx context.Context, id string, item T) (T, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (T, error)
	List(ctx context.Context, query models.PageQuery) ([]T, error)
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[T], error)
	SetStatus(ctx context.Context, id string, enabled bool) (models.ResourceStatus, error)
}

type Repository[T any] struct {
	db            *gorm.DB
	defaultSort   string
	allowedSorts  map[string]string
	allowedFilter map[string]string
}

func NewRepository[T any](db *gorm.DB, opts ...Option[T]) *Repository[T] {
	r := &Repository[T]{
		db:            db,
		defaultSort:   "created_at desc",
		allowedSorts:  defaultSorts(),
		allowedFilter: defaultFilters(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

type Option[T any] func(*Repository[T])

func WithDefaultSort[T any](sort string) Option[T] {
	return func(r *Repository[T]) {
		if strings.TrimSpace(sort) != "" {
			r.defaultSort = sort
		}
	}
}

func WithAllowedSorts[T any](sorts map[string]string) Option[T] {
	return func(r *Repository[T]) {
		for k, v := range sorts {
			if r.allowedSorts == nil {
				r.allowedSorts = map[string]string{}
			}
			r.allowedSorts[k] = v
		}
	}
}

func WithAllowedFilters[T any](filters map[string]string) Option[T] {
	return func(r *Repository[T]) {
		for k, v := range filters {
			if r.allowedFilter == nil {
				r.allowedFilter = map[string]string{}
			}
			r.allowedFilter[k] = v
		}
	}
}

func (r *Repository[T]) Create(ctx context.Context, item T) (T, error) {
	if err := r.db.WithContext(ctx).Create(&item).Error; err != nil {
		var zero T
		return zero, err
	}
	return item, nil
}

func (r *Repository[T]) Update(ctx context.Context, id string, item T) (T, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		var zero T
		return zero, ErrInvalidID
	}
	var existing T
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&existing).Error; err != nil {
		var zero T
		return zero, mapGormError(err)
	}
	copyPersistenceFields(&item, existing, id)
	if err := r.db.WithContext(ctx).Save(&item).Error; err != nil {
		var zero T
		return zero, err
	}
	return item, nil
}

func (r *Repository[T]) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidID
	}
	var item T
	tx := r.db.WithContext(ctx).Where("id = ?", id).Delete(&item)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository[T]) GetByID(ctx context.Context, id string) (T, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		var zero T
		return zero, ErrInvalidID
	}
	var item T
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		var zero T
		return zero, mapGormError(err)
	}
	return item, nil
}

func (r *Repository[T]) List(ctx context.Context, query models.PageQuery) ([]T, error) {
	var items []T
	err := r.applyQuery(r.db.WithContext(ctx).Model(new(T)), query, false).Find(&items).Error
	return items, err
}

func (r *Repository[T]) Page(ctx context.Context, query models.PageQuery) (models.PageResult[T], error) {
	query = normalizePageQuery(query)
	db := r.applyQuery(r.db.WithContext(ctx).Model(new(T)), query, true)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return models.PageResult[T]{}, err
	}

	var items []T
	offset := (query.Page - 1) * query.PageSize
	if err := r.applyOrder(db, query).Offset(offset).Limit(query.PageSize).Find(&items).Error; err != nil {
		return models.PageResult[T]{}, err
	}
	return models.PageResult[T]{
		Items:    items,
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    int(total),
	}, nil
}

func (r *Repository[T]) SetStatus(ctx context.Context, id string, enabled bool) (models.ResourceStatus, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return models.ResourceStatus{}, ErrInvalidID
	}
	var item T
	tx := r.db.WithContext(ctx).Model(&item).Where("id = ?", id).Update("enabled", enabled)
	if tx.Error != nil {
		return models.ResourceStatus{}, tx.Error
	}
	if tx.RowsAffected == 0 {
		return models.ResourceStatus{ID: id, Exists: false}, ErrNotFound
	}
	return models.ResourceStatus{ID: id, Exists: true, Enabled: &enabled}, nil
}

func (r *Repository[T]) applyQuery(db *gorm.DB, query models.PageQuery, count bool) *gorm.DB {
	if query.Enabled != nil {
		db = db.Where("enabled = ?", *query.Enabled)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		db = db.Where("name LIKE ?", "%"+strings.ReplaceAll(search, "%", "\\%")+"%")
	}
	for key, value := range query.Filters {
		column, ok := r.allowedFilter[key]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		db = db.Where(fmt.Sprintf("%s = ?", column), value)
	}
	if count {
		return db
	}
	return r.applyOrder(db, query)
}

func (r *Repository[T]) applyOrder(db *gorm.DB, query models.PageQuery) *gorm.DB {
	sortBy := strings.TrimSpace(query.SortBy)
	if sortBy == "" {
		return db.Order(r.defaultSort)
	}
	column, ok := r.allowedSorts[sortBy]
	if !ok || !validOrderName.MatchString(column) {
		return db.Order(r.defaultSort)
	}
	order := strings.ToLower(strings.TrimSpace(query.SortOrder))
	if order != "asc" {
		order = "desc"
	}
	return db.Order(column + " " + order)
}

func normalizePageQuery(query models.PageQuery) models.PageQuery {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 200 {
		query.PageSize = 200
	}
	return query
}

func mapGormError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func copyPersistenceFields[T any](item *T, existing T, id string) {
	value := reflect.ValueOf(item)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return
	}
	elem := value.Elem()
	if elem.Kind() != reflect.Struct {
		return
	}
	field := elem.FieldByName("BaseModel")
	existingValue := reflect.ValueOf(existing)
	if existingValue.Kind() == reflect.Pointer {
		existingValue = existingValue.Elem()
	}
	if field.IsValid() && field.CanSet() && existingValue.Kind() == reflect.Struct {
		existingField := existingValue.FieldByName("BaseModel")
		if existingField.IsValid() && existingField.Type().AssignableTo(field.Type()) {
			field.Set(existingField)
			if field.CanAddr() {
				if base, ok := field.Addr().Interface().(*models.BaseModel); ok {
					base.ID = id
				}
			}
			return
		}
	}
	idField := elem.FieldByName("ID")
	if idField.IsValid() && idField.CanSet() && idField.Kind() == reflect.String {
		idField.SetString(id)
	}
}

func defaultSorts() map[string]string {
	return map[string]string{
		"id":        "id",
		"name":      "name",
		"createdAt": "created_at",
		"updatedAt": "updated_at",
		"position":  "position",
	}
}

func defaultFilters() map[string]string {
	return map[string]string{
		"id":        "id",
		"name":      "name",
		"protocol":  "protocol",
		"transport": "transport",
		"type":      "type",
		"source":    "source",
		"status":    "status",
		"roleId":    "role_id",
		"agentId":   "agent_id",
	}
}
