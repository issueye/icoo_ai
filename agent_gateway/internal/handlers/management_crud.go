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

func handleCRUDAction[T any](
	c *httpx.Context,
	action string,
	create func(context.Context, T) (T, error),
	update func(context.Context, T) (T, error),
	deleteFn func(context.Context, string) error,
	pageFn func(context.Context, models.PageQuery) (models.PageResult[T], error),
	list func(context.Context) ([]T, error),
	getByID func(context.Context, string) (T, error),
	status func(context.Context, string) (models.ResourceStatus, error),
) {
	r := c.Request
	switch action {
	case "create":
		var req T
		if decodeOr400(c, &req) {
			out, err := create(r.Context(), req)
			writeCRUDResult(c, http.StatusCreated, out, err)
		}
	case "update":
		var req T
		if decodeOr400(c, &req) {
			out, err := update(r.Context(), req)
			writeCRUDResult(c, http.StatusOK, out, err)
		}
	case "delete":
		id, ok := idFromRequest(c)
		if ok {
			err := deleteFn(r.Context(), id)
			writeCRUDResult(c, http.StatusOK, map[string]string{"id": id}, err)
		}
	case "page":
		out, err := pageFn(r.Context(), pageQuery(r))
		writeCRUDResult(c, http.StatusOK, out, err)
	case "list":
		out, err := list(r.Context())
		writeCRUDResult(c, http.StatusOK, out, err)
	case "getById":
		id, ok := idFromRequest(c)
		if ok {
			out, err := getByID(r.Context(), id)
			writeCRUDResult(c, http.StatusOK, out, err)
		}
	case "status":
		id, ok := idFromRequest(c)
		if ok {
			out, err := status(r.Context(), id)
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

func decodeOr400(c *httpx.Context, dst any) bool {
	if err := decodeJSON(c, dst); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func writeCRUDResult(c *httpx.Context, status int, value any, err error) {
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
