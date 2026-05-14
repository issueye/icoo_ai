package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
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

type CRUDController[T any] struct {
	service admin.CRUDService[T]
}

type APIResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func NewCRUDController[T any](service admin.CRUDService[T]) *CRUDController[T] {
	return &CRUDController[T]{service: service}
}

func (ctl *CRUDController[T]) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("", ctl.Create)
	group.GET("", ctl.Page)
	group.GET("/:id", ctl.GetByID)
	group.PUT("/:id", ctl.Update)
	group.PATCH("/:id", ctl.Update)
	group.DELETE("/:id", ctl.Delete)
	group.PATCH("/:id/status", ctl.SetStatus)
}

func (ctl *CRUDController[T]) Create(c *gin.Context) {
	var req T
	if !bindJSON(c, &req) {
		return
	}
	out, err := ctl.service.Create(c.Request.Context(), req)
	writeResult(c, http.StatusCreated, out, err)
}

func (ctl *CRUDController[T]) Update(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req T
	if !bindJSON(c, &req) {
		return
	}
	out, err := ctl.service.Update(c.Request.Context(), id, req)
	writeResult(c, http.StatusOK, out, err)
}

func (ctl *CRUDController[T]) Delete(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	err := ctl.service.Delete(c.Request.Context(), id)
	writeResult(c, http.StatusOK, gin.H{"id": id}, err)
}

func (ctl *CRUDController[T]) GetByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	out, err := ctl.service.GetByID(c.Request.Context(), id)
	writeResult(c, http.StatusOK, out, err)
}

func (ctl *CRUDController[T]) List(c *gin.Context) {
	out, err := ctl.service.List(c.Request.Context(), pageQuery(c))
	writeResult(c, http.StatusOK, out, err)
}

func (ctl *CRUDController[T]) Page(c *gin.Context) {
	out, err := ctl.service.Page(c.Request.Context(), pageQuery(c))
	writeResult(c, http.StatusOK, out, err)
}

func (ctl *CRUDController[T]) SetStatus(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req models.StatusUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := ctl.service.SetStatus(c.Request.Context(), id, req.Enabled)
	writeResult(c, http.StatusOK, out, err)
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func writeResult(c *gin.Context, status int, data any, err error) {
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(status, APIResponse{Code: "ok", Data: data})
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, admin.ErrInvalidID):
		writeError(c, http.StatusBadRequest, "invalid_id", "id is required")
	case errors.Is(err, admin.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "resource not found")
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func writeError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, APIResponse{Code: code, Message: message})
}

func pageQuery(c *gin.Context) models.PageQuery {
	query := models.PageQuery{
		Page:      intQuery(c, "page", 1),
		PageSize:  intQuery(c, "pageSize", 20),
		SortBy:    strings.TrimSpace(c.Query("sortBy")),
		SortOrder: strings.TrimSpace(c.Query("sortOrder")),
		Search:    strings.TrimSpace(c.Query("search")),
		Filters:   map[string]string{},
	}
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			query.Enabled = &enabled
		}
	}
	for key, values := range c.Request.URL.Query() {
		if !strings.HasPrefix(key, "filter.") || len(values) == 0 {
			continue
		}
		filterKey := strings.TrimPrefix(key, "filter.")
		query.Filters[filterKey] = values[0]
	}
	return query
}

func intQuery(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
