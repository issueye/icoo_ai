package bridge

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/gatewayclient"
)

const (
	gatewayBootstrapWaitTimeout  = 12 * time.Second
	gatewayBootstrapPollInterval = 250 * time.Millisecond
	gatewayHealthTimeout         = 2 * time.Second
)

type gatewayBootstrapper struct {
	discoveryPath string
	devMode       bool
	waitTimeout   time.Duration
	pollInterval  time.Duration
	now           func() time.Time
	sleep         func(time.Duration)
	discover      func(path string) (gatewayclient.Endpoint, string, error)
	healthCheck   func(ctx context.Context, endpoint gatewayclient.Endpoint, token string) error
	startProcess  func(ctx context.Context) error
}

func newGatewayBootstrapper() *gatewayBootstrapper {
	discoveryPath := strings.TrimSpace(os.Getenv("ICOO_GATEWAY_DISCOVERY_PATH"))
	devMode := shouldEnableDevFallback()
	bootstrapper := &gatewayBootstrapper{
		discoveryPath: discoveryPath,
		devMode:       devMode,
		waitTimeout:   gatewayBootstrapWaitTimeout,
		pollInterval:  gatewayBootstrapPollInterval,
		now:           time.Now,
		sleep:         time.Sleep,
		discover:      gatewayclient.DiscoverFromPath,
	}
	bootstrapper.healthCheck = bootstrapper.defaultHealthCheck
	bootstrapper.startProcess = func(ctx context.Context) error {
		return defaultStartGatewayProcess(ctx, devMode)
	}
	return bootstrapper
}

func (b *gatewayBootstrapper) EnsureRunning(ctx context.Context) (*gatewayProxy, error) {
	proxy, err := b.discoverHealthy(ctx)
	if err == nil {
		return proxy, nil
	}
	lastErr := err

	if err := b.startProcess(ctx); err != nil {
		return nil, fmt.Errorf("start agent_gateway process: %w", err)
	}

	deadline := b.now().Add(b.waitTimeout)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		proxy, err := b.discoverHealthy(ctx)
		if err == nil {
			return proxy, nil
		}
		lastErr = err
		if !b.now().Before(deadline) {
			return nil, fmt.Errorf("gateway bootstrap timeout after %s: %w", b.waitTimeout, lastErr)
		}
		b.sleep(b.pollInterval)
	}
}

func (b *gatewayBootstrapper) discoverHealthy(ctx context.Context) (*gatewayProxy, error) {
	endpoint, token, err := b.discover(b.discoveryPath)
	if err != nil {
		return nil, err
	}
	if err := b.healthCheck(ctx, endpoint, token); err != nil {
		return nil, err
	}
	return &gatewayProxy{
		client:  http.DefaultClient,
		baseURL: strings.TrimRight(endpoint.BaseURL, "/"),
		token:   strings.TrimSpace(token),
	}, nil
}

func (b *gatewayBootstrapper) defaultHealthCheck(ctx context.Context, endpoint gatewayclient.Endpoint, token string) error {
	healthCtx := ctx
	cancel := func() {}
	if gatewayHealthTimeout > 0 {
		healthCtx, cancel = context.WithTimeout(ctx, gatewayHealthTimeout)
	}
	defer cancel()
	_, err := gatewayclient.New(endpoint.BaseURL, token).Health(healthCtx)
	return err
}

type gatewayCommandSpec struct {
	command string
	args    []string
	dir     string
}

func defaultStartGatewayProcess(ctx context.Context, devMode bool) error {
	spec, err := resolveGatewayCommand(devMode)
	if err != nil {
		return err
	}
	logPath, logFile, err := openGatewayBootstrapLog()
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, spec.command, spec.args...)
	cmd.Dir = spec.dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s %s failed: %w (log: %s)", spec.command, strings.Join(spec.args, " "), err, logPath)
	}
	return nil
}

func resolveGatewayCommand(devMode bool) (gatewayCommandSpec, error) {
	if bin := strings.TrimSpace(os.Getenv("ICOO_GATEWAY_BIN")); bin != "" {
		return gatewayCommandSpec{command: bin}, nil
	}

	names := []string{"agent-gateway"}
	if runtime.GOOS == "windows" {
		names = []string{"agent-gateway.exe"}
	}

	for _, name := range names {
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), name)
			if fileExists(candidate) {
				return gatewayCommandSpec{command: candidate}, nil
			}
		}
	}

	cwd, _ := os.Getwd()
	for _, name := range names {
		candidates := []string{
			filepath.Join(cwd, "agent_gateway", "dist", name),
			filepath.Join(cwd, "..", "agent_gateway", "dist", name),
		}
		for _, candidate := range candidates {
			if fileExists(candidate) {
				return gatewayCommandSpec{command: candidate}, nil
			}
		}
	}

	if !devMode {
		return gatewayCommandSpec{}, fmt.Errorf("agent_gateway binary not found; set ICOO_GATEWAY_BIN")
	}

	if _, err := exec.LookPath("go"); err != nil {
		return gatewayCommandSpec{}, fmt.Errorf("go command is required for dev bootstrap fallback: %w", err)
	}
	repoRoot, err := resolveRepoRoot()
	if err != nil {
		return gatewayCommandSpec{}, fmt.Errorf("resolve repo root for go run fallback: %w", err)
	}
	return gatewayCommandSpec{
		command: "go",
		args:    []string{"run", "./agent_gateway/cmd/agent-gateway"},
		dir:     repoRoot,
	}, nil
}

func resolveRepoRoot() (string, error) {
	paths := make([]string, 0, 2)
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		paths = append(paths, cwd)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		paths = append(paths, filepath.Dir(exe))
	}

	for _, start := range paths {
		root := findRepoRoot(start)
		if root != "" {
			return root, nil
		}
	}
	return "", fmt.Errorf("repository root not found")
}

func findRepoRoot(start string) string {
	current := start
	for {
		if fileExists(filepath.Join(current, "agent_gateway", "go.mod")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func openGatewayBootstrapLog() (string, *os.File, error) {
	dir := filepath.Join(os.TempDir(), "icoo-ai")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "agent-gateway-bootstrap.log")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", nil, err
	}
	return path, file, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
