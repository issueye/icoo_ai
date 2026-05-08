package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type ReactLoopOptions struct {
	Provider      llm.Provider
	Tools         []tools.Tool
	MaxToolRounds int
}

type ReactLoop struct {
	provider      llm.Provider
	tools         map[string]tools.Tool
	maxToolRounds int
}

func NewReactLoop(opts ReactLoopOptions) (*ReactLoop, error) {
	if opts.Provider == nil {
		return nil, fmt.Errorf("react loop requires provider")
	}
	maxToolRounds := opts.MaxToolRounds
	if maxToolRounds <= 0 {
		maxToolRounds = 8
	}
	toolMap := make(map[string]tools.Tool, len(opts.Tools))
	for _, tool := range opts.Tools {
		toolMap[tool.Name()] = tool
	}
	return &ReactLoop{
		provider:      opts.Provider,
		tools:         toolMap,
		maxToolRounds: maxToolRounds,
	}, nil
}

func (l *ReactLoop) Name() string {
	return "react"
}

func (l *ReactLoop) Run(ctx context.Context, req RunRequest) (<-chan Event, error) {
	out := make(chan Event)
	go func() {
		defer close(out)
		l.run(ctx, req, out)
	}()
	return out, nil
}

func (l *ReactLoop) run(ctx context.Context, req RunRequest, out chan<- Event) {
	emit(ctx, out, Event{Type: EventRunStarted, SessionID: req.SessionID})

	messages := append([]llm.Message(nil), req.Messages...)
	toolDefs := l.toolDefinitions()
	for round := 0; round <= l.maxToolRounds; round++ {
		stream, err := l.provider.Stream(ctx, llm.CompletionRequest{
			Model:    req.Options.Model,
			Messages: messages,
			Tools:    toolDefs,
		})
		if err != nil {
			emit(ctx, out, Event{Type: EventRunFailed, SessionID: req.SessionID, Error: err.Error()})
			return
		}

		var assistantText string
		var toolCalls []tools.ToolCall
		for event := range stream {
			if ctx.Err() != nil {
				emit(ctx, out, Event{Type: EventRunCancelled, SessionID: req.SessionID, Error: ctx.Err().Error()})
				return
			}
			switch event.Type {
			case llm.CompletionEventMessageDelta:
				assistantText += event.Delta
				emit(ctx, out, Event{Type: EventMessageDelta, SessionID: req.SessionID, Content: event.Delta})
			case llm.CompletionEventToolCall:
				if event.ToolCall != nil {
					toolCalls = append(toolCalls, *event.ToolCall)
				}
			case llm.CompletionEventFailed:
				emit(ctx, out, Event{Type: EventRunFailed, SessionID: req.SessionID, Error: event.Error})
				return
			}
		}

		if len(toolCalls) == 0 {
			emit(ctx, out, Event{Type: EventRunCompleted, SessionID: req.SessionID})
			return
		}

		if assistantText != "" {
			messages = append(messages, llm.Message{Role: "assistant", Content: assistantText})
		}
		for _, call := range toolCalls {
			result, ok := l.executeTool(ctx, req.SessionID, call, out)
			if !ok {
				return
			}
			resultBytes, _ := json.Marshal(result)
			messages = append(messages, llm.Message{
				Role:    "tool",
				Content: string(resultBytes),
				Metadata: map[string]any{
					"tool_call_id": call.ID,
					"tool_name":    call.Name,
				},
			})
		}
	}

	emit(ctx, out, Event{Type: EventRunFailed, SessionID: req.SessionID, Error: "maximum tool rounds exceeded"})
}

func (l *ReactLoop) executeTool(ctx context.Context, sessionID string, call tools.ToolCall, out chan<- Event) (tools.ToolResult, bool) {
	emit(ctx, out, Event{
		Type:      EventToolCallStarted,
		SessionID: sessionID,
		Data: map[string]any{
			"id":   call.ID,
			"name": call.Name,
		},
	})

	tool, ok := l.tools[call.Name]
	if !ok {
		result := tools.ToolResult{OK: false, Error: fmt.Sprintf("unknown tool %q", call.Name)}
		emit(ctx, out, Event{Type: EventToolCallCompleted, SessionID: sessionID, Error: result.Error})
		return result, true
	}

	result, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		result = tools.ToolResult{OK: false, Error: err.Error()}
	}
	emit(ctx, out, Event{
		Type:      EventToolCallCompleted,
		SessionID: sessionID,
		Content:   result.Content,
		Error:     result.Error,
		Data: map[string]any{
			"id":     call.ID,
			"name":   call.Name,
			"ok":     result.OK,
			"result": result.Data,
		},
	})
	return result, true
}

func (l *ReactLoop) toolDefinitions() []tools.ToolDefinition {
	defs := make([]tools.ToolDefinition, 0, len(l.tools))
	for _, tool := range l.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

func emit(ctx context.Context, out chan<- Event, event Event) bool {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	select {
	case <-ctx.Done():
		return false
	case out <- event:
		return true
	}
}
