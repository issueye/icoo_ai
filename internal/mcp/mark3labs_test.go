package mcp

import (
	"context"
	"errors"
	"testing"
)

func TestMark3LabsClientFactoryUnsupportedTransport(t *testing.T) {
	_, err := Mark3LabsClientFactory{}.NewClient(context.Background(), ServerDefinition{Name: "remote", Transport: TransportHTTP, URL: "https://example.com"})
	if !errors.Is(err, ErrUnsupportedTransport) {
		t.Fatalf("NewClient() error = %v, want ErrUnsupportedTransport", err)
	}
}

func TestMark3LabsClientFactoryRequiresCommand(t *testing.T) {
	_, err := Mark3LabsClientFactory{}.NewClient(context.Background(), ServerDefinition{Name: "local", Transport: TransportStdio})
	if err == nil {
		t.Fatal("NewClient() error = nil, want missing command")
	}
}
