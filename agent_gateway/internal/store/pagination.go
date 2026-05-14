package store

import "github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"

func enabledStatus(id string, enabled bool) models.ResourceStatus {
	return models.ResourceStatus{ID: id, Exists: true, Enabled: &enabled}
}

func page[T any](items []T, query models.PageQuery) models.PageResult[T] {
	pageNo := query.Page
	if pageNo <= 0 {
		pageNo = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (pageNo - 1) * pageSize
	if start >= len(items) {
		return models.PageResult[T]{Items: []T{}, Page: pageNo, PageSize: pageSize, Total: len(items)}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return models.PageResult[T]{Items: append([]T(nil), items[start:end]...), Page: pageNo, PageSize: pageSize, Total: len(items)}
}
