package bridge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/gatewayclient"
)

const (
	gatewayBootstrapWaitTimeout  = 12 * time.Second
	gatewayBootstrapPollInterval = 250 * time.Millisecond
	gatewayHealthTimeout         = 2 * time.Second
)

type gatewayBootstrapper struct {
	discoveryPath    string
	devMode          bool
	waitTimeout      time.Duration
	pollInterval     time.Duration
	now              func() time.Time
	sleep            func(time.Duration)
	discover         func(path string) (gatewayclient.Endpoint, string, error)
	healthCheck      func(ctx context.Context, endpoint gatewayclient.Endpoint, token string) error
	startProcess     func(ctx context.Context) (*os.Process, error)
	stopProcess      func(process *os.Process) error
	stopProcessByPID func(pid int) error

	processMu      sync.Mutex
	managedProcess *os.Process
	managedPID     int
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
	bootstrapper.startProcess = func(ctx context.Context) (*os.Process, error) {
		return defaultStartGatewayProcess(ctx, devMode)
	}
	bootstrapper.stopProcess = defaultStopGatewayProcess
	bootstrapper.stopProcessByPID = defaultStopGatewayProcessByPID
	return bootstrapper
}

func (b *gatewayBootstrapper) EnsureRunning(ctx context.Context) (*gatewayProxy, error) {
	proxy, _, err := b.discoverHealthy(ctx)
	if err == nil {
		return proxy, nil
	}
	lastErr := err

	startedProcess, err := b.startProcess(ctx)
	if err != nil {
		return nil, fmt.Errorf("start agent_gateway process: %w", err)
	}
	if startedProcess != nil {
		b.setManagedProcess(startedProcess)
	}

	deadline := b.now().Add(b.waitTimeout)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		proxy, endpoint, err := b.discoverHealthy(ctx)
		if err == nil {
			if endpoint.PID > 0 {
				b.setManagedPID(endpoint.PID)
			}
			return proxy, nil
		}
		lastErr = err
		if !b.now().Before(deadline) {
			_ = b.StopManagedProcess()
			return nil, fmt.Errorf("gateway bootstrap timeout after %s: %w", b.waitTimeout, lastErr)
		}
		b.sleep(b.pollInterval)
	}
}

func (b *gatewayBootstrapper) setManagedProcess(process *os.Process) {
	if b == nil || process == nil {
		return
	}
	b.processMu.Lock()
	b.managedProcess = process
	b.processMu.Unlock()
}

func (b *gatewayBootstrapper) setManagedPID(pid int) {
	if b == nil || pid <= 0 {
		return
	}
	b.processMu.Lock()
	b.managedPID = pid
	b.processMu.Unlock()
}

func (b *gatewayBootstrapper) StopManagedProcess() error {
	if b == nil {
		return nil
	}
	b.processMu.Lock()
	process := b.managedProcess
	b.managedProcess = nil
	pid := b.managedPID
	b.managedPID = 0
	stopFn := b.stopProcess
	stopByPIDFn := b.stopProcessByPID
	b.processMu.Unlock()
	if process == nil && pid <= 0 {
		return nil
	}
	if stopFn == nil {
		stopFn = defaultStopGatewayProcess
	}
	if stopByPIDFn == nil {
		stopByPIDFn = defaultStopGatewayProcessByPID
	}
	if pid > 0 {
		if err := stopByPIDFn(pid); err != nil {
			return err
		}
		return nil
	}
	if err := stopFn(process); err != nil {
		return err
	}
	return nil
}

func (b *gatewayBootstrapper) discoverHealthy(ctx context.Context) (*gatewayProxy, gatewayclient.Endpoint, error) {
	endpoint, token, err := b.discover(b.discoveryPath)
	if err != nil {
		return nil, gatewayclient.Endpoint{}, err
	}
	if err := b.healthCheck(ctx, endpoint, token); err != nil {
		return nil, gatewayclient.Endpoint{}, err
	}
	return &gatewayProxy{
		client:  http.DefaultClient,
		baseURL: strings.TrimRight(endpoint.BaseURL, "/"),
		token:   strings.TrimSpace(token),
	}, endpoint, nil
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

func defaultStartGatewayProcess(ctx context.Context, devMode bool) (*os.Process, error) {
	spec, err := resolveGatewayCommand(devMode)
	if err != nil {
		return nil, err
	}
	logPath, logFile, err := openGatewayBootstrapLog()
	if err != nil {
		return nil, err
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, spec.command, spec.args...)
	cmd.Dir = spec.dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s %s failed: %w (log: %s)", spec.command, strings.Join(spec.args, " "), err, logPath)
	}
	return cmd.Process, nil
}

func defaultStopGatewayProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func defaultStopGatewayProcessByPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return defaultStopGatewayProcess(process)
}

func resolveGatewayCommand(devMode bool) (gatewayCommandSpec, error) {
	settings, settingsErr := loadAppSettings()
	if settingsErr != nil {
		settings = normalizeAppSettings(AppSettings{})
	}
	launchArgs := gatewayLaunchArgsFromSettings(settings)

	if strings.TrimSpace(settings.GatewayBinaryPath) != "" {
		bin := strings.TrimSpace(settings.GatewayBinaryPath)
		return gatewayCommandSpec{command: bin, args: launchArgs}, nil
	}

	names := []string{"agent-gateway"}
	if runtime.GOOS == "windows" {
		names = []string{"agent-gateway.exe"}
	}

	for _, name := range names {
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), name)
			if fileExists(candidate) {
				return gatewayCommandSpec{command: candidate, args: launchArgs}, nil
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
				return gatewayCommandSpec{command: candidate, args: launchArgs}, nil
			}
		}
	}

	if !devMode {
		return gatewayCommandSpec{}, fmt.Errorf("agent_gateway binary not found")
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
		args:    append([]string{"run", "./agent_gateway/cmd/agent-gateway"}, launchArgs...),
		dir:     repoRoot,
	}, nil
}

func gatewayLaunchArgsFromSettings(settings AppSettings) []string {
	settings = normalizeAppSettings(settings)
	return []string{
		"-host", settings.GatewayHost,
		"-port", fmt.Sprintf("%d", settings.GatewayPort),
	}
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
