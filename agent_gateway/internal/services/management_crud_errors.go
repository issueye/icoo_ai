package services

import (
	"errors"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

const (
	MCP_NOT_FOUND_MSG  = "mcp server not found"
	MCP_NOT_FOUND_CODE = "mcp_server_not_found"
)

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return &GatewayError{Code: "not_found", Message: "resource not found"}
	case errors.Is(err, store.ErrDuplicateID):
		return &GatewayError{Code: "duplicate_id", Message: "resource id already exists"}
	default:
		return err
	}
}
