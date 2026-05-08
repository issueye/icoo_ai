package e2e

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/app"
	"github.com/icoo-ai/icoo-ai/internal/config"
	"github.com/icoo-ai/icoo-ai/internal/llm"
	protocolacp "github.com/icoo-ai/icoo-ai/internal/protocol/acp"
	"github.com/icoo-ai/icoo-ai/internal/testutil"
)

func TestACPFakeClientPromptSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workspace := t.TempDir()
	provider := testutil.NewMockLLMProvider("mock", []llm.CompletionEvent{
		{Type: llm.CompletionEventMessageDelta, Delta: "hello"},
		{Type: llm.CompletionEventMessageDelta, Delta: " acp"},
		{Type: llm.CompletionEventCompleted},
	})

	cfg := config.Default()
	cfg.Model = "gpt-4.1"
	components, err := app.Build(ctx, app.BuildOptions{
		Config:   cfg,
		CWD:      workspace,
		Home:     t.TempDir(),
		Provider: provider,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	clientConn, waitServer := startACPServer(t, components.Runtime)
	client := clientConn.client

	initResp, err := clientConn.conn.Initialize(ctx, sdk.InitializeRequest{
		ProtocolVersion: sdk.ProtocolVersionNumber,
		ClientInfo:      &sdk.Implementation{Name: "icoo-ai-e2e", Version: "test"},
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if initResp.ProtocolVersion != sdk.ProtocolVersionNumber {
		t.Fatalf("ProtocolVersion = %d, want %d", initResp.ProtocolVersion, sdk.ProtocolVersionNumber)
	}
	if initResp.AgentInfo == nil || initResp.AgentInfo.Name != "icoo-ai" {
		t.Fatalf("AgentInfo = %#v, want icoo-ai", initResp.AgentInfo)
	}

	sessionResp, err := clientConn.conn.NewSession(ctx, sdk.NewSessionRequest{
		Cwd:        workspace,
		McpServers: []sdk.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if sessionResp.SessionId == "" {
		t.Fatal("NewSession() returned empty session id")
	}

	promptResp, err := clientConn.conn.Prompt(ctx, sdk.PromptRequest{
		SessionId: sessionResp.SessionId,
		Prompt:    []sdk.ContentBlock{sdk.TextBlock("say hello over acp")},
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if promptResp.StopReason != sdk.StopReasonEndTurn {
		t.Fatalf("StopReason = %q, want %q", promptResp.StopReason, sdk.StopReasonEndTurn)
	}

	if got := client.agentMessageText(); got != "hello acp" {
		t.Fatalf("agent message text = %q, want %q", got, "hello acp")
	}
	if got := client.thoughtText(); got == "" {
		t.Fatal("thought updates are empty, want run lifecycle updates")
	}

	lastCall, ok := provider.LastCall()
	if !ok {
		t.Fatal("mock provider was not called")
	}
	if lastCall.Model != "gpt-4.1" {
		t.Fatalf("provider model = %q, want gpt-4.1", lastCall.Model)
	}
	if len(lastCall.Messages) == 0 || lastCall.Messages[len(lastCall.Messages)-1].Content != "say hello over acp" {
		t.Fatalf("provider messages = %#v, want prompt text forwarded", lastCall.Messages)
	}

	closeClientConn(t, clientConn)
	waitServer()
}

type acpTestConn struct {
	conn   *sdk.ClientSideConnection
	client *fakeACPClient
	c2aR   *io.PipeReader
	c2aW   *io.PipeWriter
	a2cR   *io.PipeReader
	a2cW   *io.PipeWriter
}

func startACPServer(t *testing.T, runtime agent.Runtime) (*acpTestConn, func()) {
	t.Helper()

	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	client := &fakeACPClient{}
	clientConn := sdk.NewClientSideConnection(client, c2aW, a2cR)

	server, err := protocolacp.NewServer(protocolacp.ServerOptions{
		Runtime: runtime,
		Input:   c2aR,
		Output:  a2cW,
		Name:    "icoo-ai",
		Version: "test",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve()
	}()

	conn := &acpTestConn{
		conn:   clientConn,
		client: client,
		c2aR:   c2aR,
		c2aW:   c2aW,
		a2cR:   a2cR,
		a2cW:   a2cW,
	}
	t.Cleanup(func() {
		closeClientConn(t, conn)
	})

	waitServer := func() {
		t.Helper()
		select {
		case err := <-serverErr:
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("ACP server did not stop after pipes closed")
		}
	}
	return conn, waitServer
}

func closeClientConn(t *testing.T, conn *acpTestConn) {
	t.Helper()
	_ = conn.c2aW.Close()
	_ = conn.a2cW.Close()
	_ = conn.c2aR.Close()
	_ = conn.a2cR.Close()
}

type fakeACPClient struct {
	mu      sync.Mutex
	updates []sdk.SessionNotification
}

var _ sdk.Client = (*fakeACPClient)(nil)

func (c *fakeACPClient) SessionUpdate(ctx context.Context, params sdk.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, params)
	return nil
}

func (c *fakeACPClient) RequestPermission(ctx context.Context, params sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error) {
	if len(params.Options) == 0 {
		return sdk.RequestPermissionResponse{
			Outcome: sdk.RequestPermissionOutcome{Cancelled: &sdk.RequestPermissionOutcomeCancelled{}},
		}, nil
	}
	return sdk.RequestPermissionResponse{
		Outcome: sdk.RequestPermissionOutcome{Selected: &sdk.RequestPermissionOutcomeSelected{OptionId: params.Options[0].OptionId}},
	}, nil
}

func (c *fakeACPClient) ReadTextFile(ctx context.Context, params sdk.ReadTextFileRequest) (sdk.ReadTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return sdk.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	return sdk.ReadTextFileResponse{Content: ""}, nil
}

func (c *fakeACPClient) WriteTextFile(ctx context.Context, params sdk.WriteTextFileRequest) (sdk.WriteTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return sdk.WriteTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	return sdk.WriteTextFileResponse{}, nil
}

func (c *fakeACPClient) CreateTerminal(ctx context.Context, params sdk.CreateTerminalRequest) (sdk.CreateTerminalResponse, error) {
	return sdk.CreateTerminalResponse{TerminalId: "t-1"}, nil
}

func (c *fakeACPClient) KillTerminal(ctx context.Context, params sdk.KillTerminalRequest) (sdk.KillTerminalResponse, error) {
	return sdk.KillTerminalResponse{}, nil
}

func (c *fakeACPClient) TerminalOutput(ctx context.Context, params sdk.TerminalOutputRequest) (sdk.TerminalOutputResponse, error) {
	return sdk.TerminalOutputResponse{Output: "ok"}, nil
}

func (c *fakeACPClient) ReleaseTerminal(ctx context.Context, params sdk.ReleaseTerminalRequest) (sdk.ReleaseTerminalResponse, error) {
	return sdk.ReleaseTerminalResponse{}, nil
}

func (c *fakeACPClient) WaitForTerminalExit(ctx context.Context, params sdk.WaitForTerminalExitRequest) (sdk.WaitForTerminalExitResponse, error) {
	return sdk.WaitForTerminalExitResponse{}, nil
}

func (c *fakeACPClient) agentMessageText() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var out string
	for _, notification := range c.updates {
		chunk := notification.Update.AgentMessageChunk
		if chunk == nil || chunk.Content.Text == nil {
			continue
		}
		out += chunk.Content.Text.Text
	}
	return out
}

func (c *fakeACPClient) thoughtText() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var out string
	for _, notification := range c.updates {
		chunk := notification.Update.AgentThoughtChunk
		if chunk == nil || chunk.Content.Text == nil {
			continue
		}
		out += chunk.Content.Text.Text
	}
	return out
}
