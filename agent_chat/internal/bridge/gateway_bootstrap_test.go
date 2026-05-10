package bridge

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/gatewayclient"
)

func TestGatewayBootstrapperEnsureRunning_UsesExistingHealthyGateway(t *testing.T) {
	t.Parallel()

	discoverCalls := 0
	startCalls := 0
	b := &gatewayBootstrapper{
		discoveryPath: "",
		waitTimeout:   time.Second,
		pollInterval:  time.Millisecond,
		now:           time.Now,
		sleep:         func(time.Duration) {},
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			discoverCalls++
			return gatewayclient.Endpoint{BaseURL: "http://127.0.0.1:49152"}, "token-1", nil
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			startCalls++
			return nil, nil
		},
	}

	proxy, err := b.EnsureRunning(context.Background())
	if err != nil {
		t.Fatalf("EnsureRunning returned error: %v", err)
	}
	if proxy == nil || proxy.baseURL != "http://127.0.0.1:49152" || proxy.token != "token-1" {
		t.Fatalf("unexpected proxy: %#v", proxy)
	}
	if discoverCalls != 1 {
		t.Fatalf("discover calls = %d, want 1", discoverCalls)
	}
	if startCalls != 0 {
		t.Fatalf("start calls = %d, want 0", startCalls)
	}
}

func TestGatewayBootstrapperEnsureRunning_StartsProcessAndWaitsUntilHealthy(t *testing.T) {
	t.Parallel()

	discoverCalls := 0
	startCalls := 0
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	b := &gatewayBootstrapper{
		discoveryPath: "",
		waitTimeout:   5 * time.Second,
		pollInterval:  100 * time.Millisecond,
		now: func() time.Time {
			return now
		},
		sleep: func(d time.Duration) {
			now = now.Add(d)
		},
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			discoverCalls++
			if discoverCalls < 3 {
				return gatewayclient.Endpoint{}, "", errors.New("endpoint not found")
			}
			return gatewayclient.Endpoint{BaseURL: "http://127.0.0.1:50001"}, "token-2", nil
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			startCalls++
			return nil, nil
		},
	}

	proxy, err := b.EnsureRunning(context.Background())
	if err != nil {
		t.Fatalf("EnsureRunning returned error: %v", err)
	}
	if startCalls != 1 {
		t.Fatalf("start calls = %d, want 1", startCalls)
	}
	if proxy == nil || proxy.baseURL != "http://127.0.0.1:50001" || proxy.token != "token-2" {
		t.Fatalf("unexpected proxy: %#v", proxy)
	}
}

func TestGatewayBootstrapperEnsureRunning_TimesOutWhenGatewayNeverReady(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	b := &gatewayBootstrapper{
		discoveryPath: "",
		waitTimeout:   300 * time.Millisecond,
		pollInterval:  100 * time.Millisecond,
		now: func() time.Time {
			return now
		},
		sleep: func(d time.Duration) {
			now = now.Add(d)
		},
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			return gatewayclient.Endpoint{}, "", errors.New("still not found")
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			return nil, nil
		},
	}

	_, err := b.EnsureRunning(context.Background())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestGatewayLaunchArgsFromSettings(t *testing.T) {
	t.Parallel()

	args := gatewayLaunchArgsFromSettings(AppSettings{
		GatewayHost: "127.0.0.1",
		GatewayPort: 18888,
	})
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %#v", len(args), args)
	}
	if args[0] != "-host" || args[1] != "127.0.0.1" || args[2] != "-port" || args[3] != "18888" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestGatewayLaunchArgsFromSettings_WithACPConfig(t *testing.T) {
	t.Parallel()

	args := gatewayLaunchArgsFromSettings(AppSettings{
		GatewayHost: "127.0.0.1",
		GatewayPort: 17889,
		ACPEnabled:  true,
		ACPCommand:  "icoo-ai",
		ACPArgs:     "serve --transport stdio",
	})
	want := []string{
		"-host", "127.0.0.1",
		"-port", "17889",
		"-acp-enabled",
		"-acp-command", "icoo-ai",
		"-acp-args", "serve --transport stdio",
	}
	if len(args) != len(want) {
		t.Fatalf("args len = %d, want %d: %#v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (all=%#v)", i, args[i], want[i], args)
		}
	}
}

func TestGatewayBootstrapperStopManagedProcess_SkipsUnmanagedPID(t *testing.T) {
	t.Parallel()

	stopHandleCalls := 0
	stopPIDCalls := 0
	b := &gatewayBootstrapper{
		managedPID: 3333,
		stopProcess: func(process *os.Process) error {
			stopHandleCalls++
			return nil
		},
		stopProcessByPID: func(pid int) error {
			stopPIDCalls++
			return nil
		},
	}

	if err := b.StopManagedProcess(); err != nil {
		t.Fatalf("StopManagedProcess returned error: %v", err)
	}
	if stopHandleCalls != 0 {
		t.Fatalf("stop handle calls = %d, want 0", stopHandleCalls)
	}
	if stopPIDCalls != 0 {
		t.Fatalf("stop pid calls = %d, want 0", stopPIDCalls)
	}
}

func TestGatewayBootstrapperStopManagedProcess_StopsOwnedProcess(t *testing.T) {
	t.Parallel()

	stopHandleCalls := 0
	stopPIDCalls := 0
	b := &gatewayBootstrapper{
		managedProcess: &os.Process{Pid: 1111},
		managedPID:     2222,
		managedOwned:   true,
		stopProcess: func(process *os.Process) error {
			stopHandleCalls++
			return nil
		},
		stopProcessByPID: func(pid int) error {
			stopPIDCalls++
			return nil
		},
	}

	if err := b.StopManagedProcess(); err != nil {
		t.Fatalf("StopManagedProcess returned error: %v", err)
	}
	if stopHandleCalls != 1 {
		t.Fatalf("stop handle calls = %d, want 1", stopHandleCalls)
	}
	if stopPIDCalls != 1 {
		t.Fatalf("stop pid calls = %d, want 1", stopPIDCalls)
	}
}
