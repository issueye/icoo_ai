package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

type deleteRequest struct {
	ID string `json:"id"`
}

type CURDHandlers[T any] interface {
	Create(ctx context.Context, req T) (T, error)
	Update(ctx context.Context, req T) (T, error)
	Delete(ctx context.Context, id string) error
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[T], error)
	List(ctx context.Context) ([]T, error)
	GetByID(ctx context.Context, id string) (T, error)
	Status(ctx context.Context, id string) (models.ResourceStatus, error)
}

func handleCRUDAction[T any](
	c *httpx.Context,
	action string,
	curd CURDHandlers[T],
) {
	r := c.Request
	switch action {
	case "create":
		var req T
		if decodeOr400(c, &req) {
			out, err := curd.Create(r.Context(), req)
			writeCRUDResult(c, http.StatusCreated, out, err)
		}
	case "update":
		var req T
		if decodeOr400(c, &req) {
			out, err := curd.Update(r.Context(), req)
			writeCRUDResult(c, http.StatusOK, out, err)
		}
	case "delete":
		id, ok := idFromRequest(c)
		if ok {
			err := curd.Delete(r.Context(), id)
			writeCRUDResult(c, http.StatusOK, map[string]string{"id": id}, err)
		}
	case "page":
		out, err := curd.Page(r.Context(), pageQuery(r))
		writeCRUDResult(c, http.StatusOK, out, err)
	case "list":
		out, err := curd.List(r.Context())
		writeCRUDResult(c, http.StatusOK, out, err)
	case "getById":
		id, ok := idFromRequest(c)
		if ok {
			out, err := curd.GetByID(r.Context(), id)
			writeCRUDResult(c, http.StatusOK, out, err)
		}
	case "status":
		id, ok := idFromRequest(c)
		if ok {
			out, err := curd.Status(r.Context(), id)
			writeCRUDResult(c, http.StatusOK, out, err)
		}
	default:
		writeError(c, http.StatusNotFound, "not_found", "route not found")
	}
}

func singleAction(path string, prefix string) (string, bool) {
	parts, ok := splitPath(path, prefix)
	if !ok || len(parts) != 1 {
		return "", false
	}
	return parts[0], true
}

func decodeOr400[T any](c *httpx.Context, dst T) bool {
	if err := decodeJSON(c, dst); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func writeCRUDResult[T any](c *httpx.Context, status int, value T, err error) {
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, status, value)
}

func idFromRequest(c *httpx.Context) (string, bool) {
	r := c.Request
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" && r.Body != nil && r.ContentLength != 0 {
		var req deleteRequest
		if err := decodeJSON(c, &req); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return "", false
		}
		id = strings.TrimSpace(req.ID)
	}
	if id == "" {
		writeError(c, http.StatusBadRequest, "invalid_id", "id is required")
		return "", false
	}
	return id, true
}

func pageQuery(r *http.Request) models.PageQuery {
	return models.PageQuery{
		Page:     intQuery(r, "page", 1),
		PageSize: intQuery(r, "pageSize", 20),
	}
}

func intQuery(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
