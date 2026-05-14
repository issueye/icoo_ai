package models

type PageQuery struct {
	Page      int               `json:"page"`
	PageSize  int               `json:"pageSize"`
	SortBy    string            `json:"sortBy,omitempty"`
	SortOrder string            `json:"sortOrder,omitempty"`
	Search    string            `json:"search,omitempty"`
	Enabled   *bool             `json:"enabled,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
}

type PageResult[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

type ResourceStatus struct {
	ID      string `json:"id"`
	Exists  bool   `json:"exists"`
	Enabled *bool  `json:"enabled,omitempty"`
}

type StatusUpdateRequest struct {
	Enabled bool `json:"enabled"`
}
