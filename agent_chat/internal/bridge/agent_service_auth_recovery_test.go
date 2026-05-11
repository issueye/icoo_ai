package bridge

import (
	"context"
	"testing"
	"time"
)

func TestWaitGatewayReconnectBackoff_DoublesWithCap(t *testing.T) {
	if got := nextGatewayReconnectBackoff(100 * time.Millisecond); got != 200*time.Millisecond {
		t.Fatalf("next backoff = %s, want 200ms", got)
	}
	if got := nextGatewayReconnectBackoff(9 * time.Second); got != 10*time.Second {
		t.Fatalf("next backoff = %s, want 10s cap", got)
	}
	if got := nextGatewayReconnectBackoff(10 * time.Second); got != 10*time.Second {
		t.Fatalf("next backoff = %s, want remain 10s cap", got)
	}
}

func TestWaitGatewayReconnectBackoff_StopsOnCanceledContext(t *testing.T) {
	backoff := 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ok := waitGatewayReconnectBackoff(ctx, &backoff); ok {
		t.Fatal("waitGatewayReconnectBackoff() = true, want false when context canceled")
	}
}
