package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type Agent struct {
	ctx    context.Context
	cancel context.CancelFunc
	models.Agent
	ACPServer *acp.Agent
	cmd       *exec.Cmd
	logger    *slog.Logger
	conn      *acp.ClientSideConnection
}

type Manager struct {
	lock      sync.Mutex // 用于保护bootstrap的访问
	bootstrap []Agent    // 用于初始化的智能体配置
}

// NewManager 创建智能体管理器
func NewManager(profiles []models.Agent) *Manager {
	agents := make([]Agent, 0, len(profiles))
	for _, profile := range profiles {
		agents = append(agents, Agent{
			Agent: profile,
		})
	}

	return &Manager{
		bootstrap: agents,
	}
}

func (m *Manager) List(ctx context.Context) ([]Agent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return m.bootstrap, nil
}

func (m *Manager) Has(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || m == nil {
		return false
	}
	for _, item := range m.bootstrap {
		if item.ID == id {
			return true
		}
	}
	return false
}

// Remove 移除制定AGENT
func (m *Manager) Remove(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || m == nil {
		return false
	}
	for idx, item := range m.bootstrap {
		if item.ID == id {
			m.bootstrap = append(m.bootstrap[:idx], m.bootstrap[idx+1:]...)
			return true
		}
	}
	return false
}

func NewAgent(l *slog.Logger) (*Agent, error) {
	agent := &Agent{logger: l}

	ctx, cancel := context.WithCancel(context.Background())
	agent.ctx = ctx
	agent.cancel = cancel

	var cmd *exec.Cmd
	cmd = exec.CommandContext(ctx, agent.Command, agent.Args...)
	agent.cmd = cmd

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	err := cmd.Start()
	if err != nil {
		return agent, err
	}

	client := NewAcpClient(agent.logger)
	conn := acp.NewClientSideConnection(client, stdin, stdout)
	conn.SetLogger(agent.logger)

	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs:       acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
			Terminal: true,
		},
	})
	if err != nil {
		return agent, err
	}

	// 记录协议版本
	agent.logger.Info("协议版本", "ProtocolVersion", initResp.ProtocolVersion)

	agent.conn = conn
	return agent, nil
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func (a *Agent) SendMessage(msg string) error {
	ctx := context.Background()
	newSess, err := a.conn.NewSession(ctx, acp.NewSessionRequest{Cwd: mustCwd(), McpServers: []acp.McpServer{}})
	if err != nil {
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				fmt.Fprintf(os.Stderr, "[Client] Error: %s\n", string(b))
			} else {
				fmt.Fprintf(os.Stderr, "newSession error (%d): %s\n", re.Code, re.Message)
			}
		} else {
			fmt.Fprintf(os.Stderr, "newSession error: %v\n", err)
		}
		_ = a.cmd.Process.Kill()
		return err
	}
	_, err = a.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: newSess.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(msg)},
	})
	if err != nil {
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				fmt.Fprintf(os.Stderr, "[Client] Error: %s\n", string(b))
			} else {
				fmt.Fprintf(os.Stderr, "prompt error (%d): %s\n", re.Code, re.Message)
			}
		} else {
			fmt.Fprintf(os.Stderr, "prompt error: %v\n", err)
		}
	}

	return nil
}
