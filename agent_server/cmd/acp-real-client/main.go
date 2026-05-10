package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sdk "github.com/coder/acp-go-sdk"
)

type llmInfo struct {
	APIKey  string
	BaseURL string
	Model   string
}

type options struct {
	agentDir       string
	agentCommand   string
	agentArgs      string
	llmInfoPath    string
	workspace      string
	prompt         string
	timeoutSeconds int
}

type testClient struct {
	mu              sync.Mutex
	agentText       strings.Builder
	thoughtText     strings.Builder
	permissionCount int
}

var _ sdk.Client = (*testClient)(nil)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(parent context.Context, args []string) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}
	info, err := loadLLMInfo(opts.llmInfoPath)
	if err != nil {
		return err
	}

	timeout := time.Duration(opts.timeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	return runACPScenario(ctx, opts, info)
}

func parseFlags(args []string) (options, error) {
	wd, _ := os.Getwd()
	defaultWorkspace := wd
	defaultAgentDir := detectDefaultAgentDir(wd)
	defaultLLMInfoPath := detectDefaultLLMInfoPath(wd)

	fs := flag.NewFlagSet("acp-real-client", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := options{}
	fs.StringVar(&opts.agentDir, "agent-dir", defaultAgentDir, "agent_server directory")
	fs.StringVar(&opts.agentCommand, "agent-command", "go", "agent server startup command")
	fs.StringVar(&opts.agentArgs, "agent-args", "run ./cmd/icoo-ai serve", "agent server startup args")
	fs.StringVar(&opts.llmInfoPath, "llm-info", defaultLLMInfoPath, "llm info file path")
	fs.StringVar(&opts.workspace, "workspace", defaultWorkspace, "session workspace cwd")
	fs.StringVar(&opts.prompt, "prompt", "请只回复 ACP_OK", "prompt sent to agent")
	fs.IntVar(&opts.timeoutSeconds, "timeout", 180, "test timeout in seconds")

	if err := fs.Parse(args); err != nil {
		return options{}, fmt.Errorf("parse flags: %w", err)
	}
	if strings.TrimSpace(opts.agentCommand) == "" {
		return options{}, errors.New("agent-command is required")
	}
	if strings.TrimSpace(opts.agentArgs) == "" {
		return options{}, errors.New("agent-args is required")
	}
	if strings.TrimSpace(opts.workspace) == "" {
		return options{}, errors.New("workspace is required")
	}
	if opts.timeoutSeconds <= 0 {
		return options{}, errors.New("timeout must be positive")
	}
	return opts, nil
}

func detectDefaultAgentDir(wd string) string {
	candidates := []string{
		wd,
		filepath.Join(wd, "agent_server"),
	}
	for _, candidate := range candidates {
		mainPath := filepath.Join(candidate, "cmd", "icoo-ai", "main.go")
		if fileExists(mainPath) {
			return candidate
		}
	}
	return filepath.Join(wd, "agent_server")
}

func detectDefaultLLMInfoPath(wd string) string {
	candidates := []string{
		filepath.Join(wd, "docs", "llm_info.txt"),
		filepath.Join(wd, "..", "docs", "llm_info.txt"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return filepath.Join(wd, "docs", "llm_info.txt")
}

func loadLLMInfo(path string) (llmInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return llmInfo{}, fmt.Errorf("read llm info file %q: %w", path, err)
	}
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(strings.ToLower(key))] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return llmInfo{}, fmt.Errorf("scan llm info file %q: %w", path, err)
	}
	info := llmInfo{
		APIKey:  values["api_key"],
		BaseURL: values["base_url"],
		Model:   values["model"],
	}
	if info.APIKey == "" || info.BaseURL == "" || info.Model == "" {
		return llmInfo{}, fmt.Errorf("llm info file %q requires api_key/base_url/model", path)
	}
	return info, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func runACPScenario(ctx context.Context, opts options, info llmInfo) error {
	agentDir, err := filepath.Abs(opts.agentDir)
	if err != nil {
		return fmt.Errorf("resolve agent-dir: %w", err)
	}
	workspace, err := filepath.Abs(opts.workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}

	args := strings.Fields(opts.agentArgs)
	cmd := exec.CommandContext(ctx, opts.agentCommand, args...)
	cmd.Dir = agentDir
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"ICOO_AI_PROVIDER=openai",
		"ICOO_AI_API=responses",
		"ICOO_AI_API_KEY="+info.APIKey,
		"ICOO_AI_BASE_URL="+info.BaseURL,
		"ICOO_AI_MODEL="+info.Model,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create agent stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create agent stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start agent command failed: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	client := &testClient{}
	conn := sdk.NewClientSideConnection(client, stdin, stdout)
	conn.SetLogger(slog.Default())

	fmt.Printf("ACP test started, agent-dir=%s workspace=%s model=%s apiKey=%s\n",
		agentDir, workspace, info.Model, maskSecret(info.APIKey))

	initResp, err := conn.Initialize(ctx, sdk.InitializeRequest{
		ProtocolVersion: sdk.ProtocolVersionNumber,
		ClientCapabilities: sdk.ClientCapabilities{
			Fs:       sdk.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
			Terminal: true,
		},
	})
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}
	fmt.Printf("[OK] initialize protocol=%d agent=%s\n", initResp.ProtocolVersion, agentName(initResp))

	newSessionResp, err := conn.NewSession(ctx, sdk.NewSessionRequest{
		Cwd:        workspace,
		McpServers: []sdk.McpServer{},
	})
	if err != nil {
		return fmt.Errorf("newSession failed: %w", err)
	}
	sessionID := newSessionResp.SessionId
	fmt.Printf("[OK] newSession sessionId=%s\n", sessionID)

	listResp, err := conn.ListSessions(ctx, sdk.ListSessionsRequest{})
	if err != nil {
		return fmt.Errorf("listSessions failed: %w", err)
	}
	if !containsSession(listResp.Sessions, sessionID) {
		return fmt.Errorf("listSessions missing created session %q", sessionID)
	}
	fmt.Printf("[OK] listSessions count=%d\n", len(listResp.Sessions))

	resumeResp, err := conn.ResumeSession(ctx, sdk.ResumeSessionRequest{
		SessionId: sessionID,
		Cwd:       workspace,
	})
	if err != nil {
		return fmt.Errorf("resumeSession failed: %w", err)
	}
	fmt.Printf("[OK] resumeSession modes=%v configOptions=%d\n",
		resumeResp.Modes != nil, len(resumeResp.ConfigOptions))

	if _, err := conn.SetSessionMode(ctx, sdk.SetSessionModeRequest{
		SessionId: sessionID,
		ModeId:    sdk.SessionModeId("agent"),
	}); err != nil {
		return fmt.Errorf("setSessionMode failed: %w", err)
	}
	fmt.Println("[OK] setSessionMode mode=agent")

	if _, err := conn.SetSessionConfigOption(ctx, sdk.SetSessionConfigOptionRequest{
		ValueId: &sdk.SetSessionConfigOptionValueId{
			SessionId: sessionID,
			ConfigId:  sdk.SessionConfigId("approval_mode"),
			Value:     sdk.SessionConfigValueId("workspace-write"),
		},
	}); err != nil {
		return fmt.Errorf("setSessionConfigOption approval_mode failed: %w", err)
	}
	fmt.Println("[OK] setSessionConfigOption approval_mode=workspace-write")

	if _, err := conn.SetSessionConfigOption(ctx, sdk.SetSessionConfigOptionRequest{
		Boolean: &sdk.SetSessionConfigOptionBoolean{
			SessionId: sessionID,
			ConfigId:  sdk.SessionConfigId("emit_plan_updates"),
			Type:      "boolean",
			Value:     true,
		},
	}); err != nil {
		return fmt.Errorf("setSessionConfigOption emit_plan_updates failed: %w", err)
	}
	fmt.Println("[OK] setSessionConfigOption emit_plan_updates=true")

	promptResp, err := conn.Prompt(ctx, sdk.PromptRequest{
		SessionId: sessionID,
		Prompt:    []sdk.ContentBlock{sdk.TextBlock(opts.prompt)},
	})
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	fmt.Printf("[OK] prompt stopReason=%s\n", promptResp.StopReason)

	if _, err := conn.CloseSession(ctx, sdk.CloseSessionRequest{
		SessionId: sessionID,
	}); err != nil {
		return fmt.Errorf("closeSession failed: %w", err)
	}
	fmt.Println("[OK] closeSession")

	agentText, thoughtText, permissionCount := client.snapshot()
	fmt.Println("----- ACP Scenario Summary -----")
	fmt.Printf("sessionId: %s\n", sessionID)
	fmt.Printf("permissionRequests: %d\n", permissionCount)
	fmt.Printf("agentText: %s\n", strings.TrimSpace(agentText))
	fmt.Printf("thoughtText: %s\n", strings.TrimSpace(thoughtText))
	fmt.Println("ACP real environment test completed.")
	return nil
}

func containsSession(items []sdk.SessionInfo, sessionID sdk.SessionId) bool {
	for _, item := range items {
		if item.SessionId == sessionID {
			return true
		}
	}
	return false
}

func agentName(resp sdk.InitializeResponse) string {
	if resp.AgentInfo == nil {
		return ""
	}
	return resp.AgentInfo.Name
}

func maskSecret(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return "****"
	}
	return trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
}

func (c *testClient) RequestPermission(ctx context.Context, params sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error) {
	c.mu.Lock()
	c.permissionCount++
	c.mu.Unlock()
	if len(params.Options) == 0 {
		return sdk.RequestPermissionResponse{
			Outcome: sdk.RequestPermissionOutcome{
				Cancelled: &sdk.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}
	return sdk.RequestPermissionResponse{
		Outcome: sdk.RequestPermissionOutcome{
			Selected: &sdk.RequestPermissionOutcomeSelected{OptionId: params.Options[0].OptionId},
		},
	}, nil
}

func (c *testClient) SessionUpdate(ctx context.Context, params sdk.SessionNotification) error {
	update := params.Update
	c.mu.Lock()
	defer c.mu.Unlock()
	if chunk := update.AgentMessageChunk; chunk != nil && chunk.Content.Text != nil {
		c.agentText.WriteString(chunk.Content.Text.Text)
	}
	if chunk := update.AgentThoughtChunk; chunk != nil && chunk.Content.Text != nil {
		c.thoughtText.WriteString(chunk.Content.Text.Text)
	}
	return nil
}

func (c *testClient) ReadTextFile(ctx context.Context, params sdk.ReadTextFileRequest) (sdk.ReadTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return sdk.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	content, err := os.ReadFile(params.Path)
	if err != nil {
		return sdk.ReadTextFileResponse{}, err
	}
	return sdk.ReadTextFileResponse{Content: string(content)}, nil
}

func (c *testClient) WriteTextFile(ctx context.Context, params sdk.WriteTextFileRequest) (sdk.WriteTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return sdk.WriteTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	if err := os.MkdirAll(filepath.Dir(params.Path), 0o755); err != nil {
		return sdk.WriteTextFileResponse{}, err
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o644); err != nil {
		return sdk.WriteTextFileResponse{}, err
	}
	return sdk.WriteTextFileResponse{}, nil
}

func (c *testClient) CreateTerminal(ctx context.Context, params sdk.CreateTerminalRequest) (sdk.CreateTerminalResponse, error) {
	return sdk.CreateTerminalResponse{TerminalId: "acp-real-client-terminal"}, nil
}

func (c *testClient) KillTerminal(ctx context.Context, params sdk.KillTerminalRequest) (sdk.KillTerminalResponse, error) {
	return sdk.KillTerminalResponse{}, nil
}

func (c *testClient) TerminalOutput(ctx context.Context, params sdk.TerminalOutputRequest) (sdk.TerminalOutputResponse, error) {
	return sdk.TerminalOutputResponse{Output: "", Truncated: false}, nil
}

func (c *testClient) ReleaseTerminal(ctx context.Context, params sdk.ReleaseTerminalRequest) (sdk.ReleaseTerminalResponse, error) {
	return sdk.ReleaseTerminalResponse{}, nil
}

func (c *testClient) WaitForTerminalExit(ctx context.Context, params sdk.WaitForTerminalExitRequest) (sdk.WaitForTerminalExitResponse, error) {
	return sdk.WaitForTerminalExitResponse{}, nil
}

func (c *testClient) snapshot() (agentText string, thoughtText string, permissionCount int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.agentText.String(), c.thoughtText.String(), c.permissionCount
}
