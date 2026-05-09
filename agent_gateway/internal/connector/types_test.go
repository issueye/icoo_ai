package connector

import (
	"errors"
	"testing"
)

func TestWrapError(t *testing.T) {
	cause := errors.New("boom")
	err := WrapError("x_code", "x_message", cause)
	if err.Code != "x_code" || err.Message != "x_message" {
		t.Fatalf("unexpected wrapped error: %#v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause")
	}
}
