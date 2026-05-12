package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type deleteRequest struct {
	ID string `json:"id"`
}

func handleCRUDAction[T any](
	w http.ResponseWriter,
	r *http.Request,
	action string,
	create func(context.Context, T) (T, error),
	update func(context.Context, T) (T, error),
	deleteFn func(context.Context, string) error,
	pageFn func(context.Context, models.PageQuery) (models.PageResult[T], error),
	list func(context.Context) ([]T, error),
	getByID func(context.Context, string) (T, error),
	status func(context.Context, string) (models.ResourceStatus, error),
) {
	switch action {
	case "create":
		requireMethod(w, r, http.MethodPost, func() {
			var req T
			if decodeOr400(w, r, &req) {
				out, err := create(r.Context(), req)
				writeCRUDResult(w, http.StatusCreated, out, err)
			}
		})
	case "update":
		requireMethod(w, r, http.MethodPut, func() {
			var req T
			if decodeOr400(w, r, &req) {
				out, err := update(r.Context(), req)
				writeCRUDResult(w, http.StatusOK, out, err)
			}
		})
	case "delete":
		requireMethod(w, r, http.MethodDelete, func() {
			id, ok := idFromRequest(w, r)
			if ok {
				err := deleteFn(r.Context(), id)
				writeCRUDResult(w, http.StatusOK, map[string]string{"id": id}, err)
			}
		})
	case "page":
		requireMethod(w, r, http.MethodGet, func() {
			out, err := pageFn(r.Context(), pageQuery(r))
			writeCRUDResult(w, http.StatusOK, out, err)
		})
	case "list":
		requireMethod(w, r, http.MethodGet, func() {
			out, err := list(r.Context())
			writeCRUDResult(w, http.StatusOK, out, err)
		})
	case "getById":
		requireMethod(w, r, http.MethodGet, func() {
			id, ok := idFromRequest(w, r)
			if ok {
				out, err := getByID(r.Context(), id)
				writeCRUDResult(w, http.StatusOK, out, err)
			}
		})
	case "status":
		requireMethod(w, r, http.MethodGet, func() {
			id, ok := idFromRequest(w, r)
			if ok {
				out, err := status(r.Context(), id)
				writeCRUDResult(w, http.StatusOK, out, err)
			}
		})
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func singleAction(path string, prefix string) (string, bool) {
	parts, ok := splitPath(path, prefix)
	if !ok || len(parts) != 1 {
		return "", false
	}
	return parts[0], true
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string, next func()) {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	next()
}

func decodeOr400(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := decodeJSON(r, dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func writeCRUDResult(w http.ResponseWriter, status int, value any, err error) {
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, status, value)
}

func idFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" && r.Body != nil && r.ContentLength != 0 {
		var req deleteRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return "", false
		}
		id = strings.TrimSpace(req.ID)
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "id is required")
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
