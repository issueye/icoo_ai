package models

type PageQuery struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
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
