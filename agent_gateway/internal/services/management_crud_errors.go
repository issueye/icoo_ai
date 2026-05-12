package services

import (
	"errors"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
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
