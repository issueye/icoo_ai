package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/hooks"
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
	approved      map[string]struct{}
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
		approved:      map[string]struct{}{},
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
	if result, blocked := l.beforeRun(ctx, req); blocked {
		emit(ctx, out, Event{Type: EventRunFailed, SessionID: req.SessionID, Error: result.Error})
		return
	}
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
			l.failRun(ctx, req, out, err.Error())
			return
		}

		var assistantText string
		var toolCalls []tools.ToolCall
		for event := range stream {
			if ctx.Err() != nil {
				l.onRunError(ctx, req, ctx.Err().Error())
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
				l.failRun(ctx, req, out, event.Error)
				return
			}
		}

		if len(toolCalls) == 0 {
			l.afterRun(ctx, req, true, "")
			emit(ctx, out, Event{Type: EventRunCompleted, SessionID: req.SessionID})
			return
		}

		messages = append(messages, llm.Message{Role: "assistant", Content: assistantText, ToolCalls: toolCalls})
		for _, call := range toolCalls {
			result, ok := l.executeTool(ctx, req, call, out)
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

	l.failRun(ctx, req, out, "maximum tool rounds exceeded")
}

func (l *ReactLoop) beforeRun(ctx context.Context, req RunRequest) (tools.ToolResult, bool) {
	if req.Options.Hooks == nil {
		return tools.ToolResult{}, false
	}
	dispatch, err := req.Options.Hooks.Dispatch(ctx, hooks.Event{
		Type:      hooks.EventBeforeRun,
		SessionID: req.SessionID,
		Name:      l.Name(),
		CWD:       req.CWD,
		Data: map[string]any{
			"message_count": len(req.Messages),
			"tool_count":    len(l.tools),
			"model":         req.Options.Model,
		},
	})
	if err != nil {
		return tools.ToolResult{OK: false, Error: err.Error(), Data: map[string]any{"code": "hook_failed"}}, true
	}
	logHookDispatch(ctx, req.Options.AuditLogger, req.SessionID, dispatch)
	if dispatch.Action == hooks.ActionBlock || dispatch.Action == hooks.ActionRequestApproval {
		return hookBlockedResult("hook_blocked", dispatch), true
	}
	return tools.ToolResult{}, false
}

func (l *ReactLoop) afterRun(ctx context.Context, req RunRequest, ok bool, errText string) {
	if req.Options.Hooks == nil {
		return
	}
	dispatch, err := req.Options.Hooks.Dispatch(ctx, hooks.Event{
		Type:      hooks.EventAfterRun,
		SessionID: req.SessionID,
		Name:      l.Name(),
		CWD:       req.CWD,
		Error:     errText,
		Data: map[string]any{
			"ok": ok,
		},
	})
	if err == nil {
		logHookDispatch(ctx, req.Options.AuditLogger, req.SessionID, dispatch)
	}
}

func (l *ReactLoop) onRunError(ctx context.Context, req RunRequest, errText string) {
	if req.Options.Hooks == nil {
		return
	}
	dispatch, err := req.Options.Hooks.Dispatch(ctx, hooks.Event{
		Type:      hooks.EventOnError,
		SessionID: req.SessionID,
		Name:      l.Name(),
		CWD:       req.CWD,
		Error:     errText,
	})
	if err == nil {
		logHookDispatch(ctx, req.Options.AuditLogger, req.SessionID, dispatch)
	}
}

func (l *ReactLoop) failRun(ctx context.Context, req RunRequest, out chan<- Event, errText string) {
	l.onRunError(ctx, req, errText)
	l.afterRun(ctx, req, false, errText)
	emit(ctx, out, Event{Type: EventRunFailed, SessionID: req.SessionID, Error: errText})
}

func (l *ReactLoop) executeTool(ctx context.Context, req RunRequest, call tools.ToolCall, out chan<- Event) (tools.ToolResult, bool) {
	call, hookResult, blocked := l.beforeToolCall(ctx, req, call, out)
	if blocked {
		emit(ctx, out, Event{
			Type:      EventToolCallCompleted,
			SessionID: req.SessionID,
			Error:     hookResult.Error,
			Data: map[string]any{
				"id":     call.ID,
				"name":   call.Name,
				"ok":     hookResult.OK,
				"result": hookResult.Data,
			},
		})
		return hookResult, true
	}

	emit(ctx, out, Event{
		Type:      EventToolCallStarted,
		SessionID: req.SessionID,
		Data: map[string]any{
			"id":   call.ID,
			"name": call.Name,
		},
	})

	tool, ok := l.tools[call.Name]
	if !ok {
		result := tools.ToolResult{OK: false, Error: fmt.Sprintf("unknown tool %q", call.Name)}
		emit(ctx, out, Event{Type: EventToolCallCompleted, SessionID: req.SessionID, Error: result.Error})
		return result, true
	}

	result, err := l.executeWithApproval(ctx, req, tool, call, out)
	if err != nil {
		result = tools.ToolResult{OK: false, Error: err.Error()}
	}
	result = l.afterToolCall(ctx, req, call, result)
	emit(ctx, out, Event{
		Type:      EventToolCallCompleted,
		SessionID: req.SessionID,
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

func (l *ReactLoop) beforeToolCall(ctx context.Context, req RunRequest, call tools.ToolCall, out chan<- Event) (tools.ToolCall, tools.ToolResult, bool) {
	if req.Options.Hooks == nil {
		return call, tools.ToolResult{}, false
	}
	dispatch, err := req.Options.Hooks.Dispatch(ctx, hooks.Event{
		Type:      hooks.EventBeforeToolCall,
		SessionID: req.SessionID,
		Name:      call.Name,
		CWD:       req.CWD,
		Data: map[string]any{
			"id":        call.ID,
			"arguments": hookArguments(call.Arguments),
		},
	})
	if err != nil {
		return call, tools.ToolResult{OK: false, Error: err.Error(), Data: map[string]any{"code": "hook_failed"}}, true
	}
	logHookDispatch(ctx, req.Options.AuditLogger, req.SessionID, dispatch)
	call.Name = dispatch.Event.Name
	if raw, ok := jsonFromHookArguments(dispatch.Event.Data["arguments"]); ok {
		call.Arguments = raw
	}
	switch dispatch.Action {
	case hooks.ActionBlock:
		return call, hookBlockedResult("hook_blocked", dispatch), true
	case hooks.ActionRequestApproval:
		result, allowed := l.approveHookDecision(ctx, req, call, dispatch, out)
		return call, result, !allowed
	default:
		return call, tools.ToolResult{}, false
	}
}

func (l *ReactLoop) afterToolCall(ctx context.Context, req RunRequest, call tools.ToolCall, result tools.ToolResult) tools.ToolResult {
	if req.Options.Hooks == nil {
		return result
	}
	dispatch, err := req.Options.Hooks.Dispatch(ctx, hooks.Event{
		Type:      hooks.EventAfterToolCall,
		SessionID: req.SessionID,
		Name:      call.Name,
		CWD:       req.CWD,
		Data: map[string]any{
			"id":      call.ID,
			"ok":      result.OK,
			"content": result.Content,
			"error":   result.Error,
			"result":  result.Data,
		},
	})
	if err != nil {
		return tools.ToolResult{OK: false, Error: err.Error(), Data: map[string]any{"code": "hook_failed"}}
	}
	logHookDispatch(ctx, req.Options.AuditLogger, req.SessionID, dispatch)
	if value, ok := dispatch.Event.Data["ok"].(bool); ok {
		result.OK = value
	}
	if value, ok := dispatch.Event.Data["content"].(string); ok {
		result.Content = value
	}
	if value, ok := dispatch.Event.Data["error"].(string); ok {
		result.Error = value
	}
	if value, ok := dispatch.Event.Data["result"].(map[string]any); ok {
		result.Data = value
	}
	if dispatch.Action == hooks.ActionBlock {
		return hookBlockedResult("hook_blocked", dispatch)
	}
	return result
}

func (l *ReactLoop) approveHookDecision(ctx context.Context, req RunRequest, call tools.ToolCall, dispatch hooks.DispatchResult, out chan<- Event) (tools.ToolResult, bool) {
	if req.Options.Approver == nil {
		return hookBlockedResult("approval_required", dispatch), false
	}
	emit(ctx, out, Event{
		Type:      EventApprovalRequested,
		SessionID: req.SessionID,
		Content:   dispatch.Reason,
		Data: map[string]any{
			"id":     call.ID,
			"name":   call.Name,
			"reason": dispatch.Reason,
			"data":   dispatch.Data,
		},
	})
	decision, err := req.Options.Approver.Approve(ctx, ApprovalRequest{
		SessionID: req.SessionID,
		ToolName:  call.Name,
		ToolCall:  string(call.Arguments),
		Reason:    dispatch.Reason,
		Data:      dispatch.Data,
	})
	if err != nil || decision == ApprovalDecisionDeny {
		return hookBlockedResult("approval_denied", dispatch), false
	}
	return tools.ToolResult{}, true
}

func (l *ReactLoop) executeWithApproval(ctx context.Context, req RunRequest, tool tools.Tool, call tools.ToolCall, out chan<- Event) (tools.ToolResult, error) {
	if capable, ok := tool.(tools.ApprovalCapable); ok {
		key, ok := capable.ApprovalKey(call.Arguments)
		if ok {
			approvalKey := call.Name + "\x00" + key
			if _, always := l.approved[approvalKey]; always {
				return capable.ExecuteApproved(ctx, call.Arguments, tools.ApprovalScopeAlways)
			}
		}
	}
	result, err := tool.Execute(ctx, call.Arguments)
	if err != nil || !isApprovalRequired(result) {
		return result, err
	}
	capable, ok := tool.(tools.ApprovalCapable)
	if !ok || req.Options.Approver == nil {
		return result, err
	}
	approvalKey, _ := capable.ApprovalKey(call.Arguments)
	emit(ctx, out, Event{
		Type:      EventApprovalRequested,
		SessionID: req.SessionID,
		Content:   result.Error,
		Data: map[string]any{
			"id":     call.ID,
			"name":   call.Name,
			"reason": result.Error,
			"data":   result.Data,
		},
	})
	decision, approveErr := req.Options.Approver.Approve(ctx, ApprovalRequest{
		SessionID: req.SessionID,
		ToolName:  call.Name,
		ToolCall:  string(call.Arguments),
		Reason:    result.Error,
		Data:      result.Data,
	})
	if approveErr != nil {
		emitApprovalDecision(ctx, out, req.SessionID, call, ApprovalDecisionDeny, approveErr.Error())
		return tools.ToolResult{OK: false, Error: approveErr.Error()}, nil
	}
	emitApprovalDecision(ctx, out, req.SessionID, call, decision, "")
	switch decision {
	case ApprovalDecisionOnce:
		return capable.ExecuteApproved(ctx, call.Arguments, tools.ApprovalScopeOnce)
	case ApprovalDecisionAlways:
		if approvalKey != "" {
			l.approved[call.Name+"\x00"+approvalKey] = struct{}{}
		}
		return capable.ExecuteApproved(ctx, call.Arguments, tools.ApprovalScopeAlways)
	default:
		return tools.ToolResult{OK: false, Error: "approval denied", Data: map[string]any{"code": "approval_denied"}}, nil
	}
}

func emitApprovalDecision(ctx context.Context, out chan<- Event, sessionID string, call tools.ToolCall, decision ApprovalDecision, errText string) {
	data := map[string]any{
		"id":       call.ID,
		"name":     call.Name,
		"decision": decision,
	}
	if errText != "" {
		data["error"] = errText
	}
	emit(ctx, out, Event{Type: EventApprovalDecided, SessionID: sessionID, Error: errText, Data: data})
}

func isApprovalRequired(result tools.ToolResult) bool {
	if result.Data == nil {
		return false
	}
	code, _ := result.Data["code"].(string)
	return code == "approval_required"
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

func hookArguments(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{"raw": string(raw)}
	}
	return decoded
}

func jsonFromHookArguments(value any) (json.RawMessage, bool) {
	if value == nil {
		return nil, false
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(data), true
}

func hookBlockedResult(code string, dispatch hooks.DispatchResult) tools.ToolResult {
	data := map[string]any{
		"code":    code,
		"action":  string(dispatch.Action),
		"results": dispatch.Results,
	}
	if dispatch.Data != nil {
		data["hook_data"] = dispatch.Data
	}
	return tools.ToolResult{OK: false, Error: dispatch.Reason, Data: data}
}

func logHookDispatch(ctx context.Context, logger audit.Logger, sessionID string, dispatch hooks.DispatchResult) {
	if logger == nil {
		return
	}
	data := map[string]any{
		"event_type": dispatch.Event.Type,
		"event_name": dispatch.Event.Name,
		"action":     dispatch.Action,
		"results":    dispatch.Results,
	}
	if dispatch.Reason != "" {
		data["reason"] = dispatch.Reason
	}
	_ = logger.Log(ctx, audit.Event{
		Type:      audit.EventHookUse,
		SessionID: sessionID,
		Timestamp: time.Now().UTC(),
		Summary:   "hook " + string(dispatch.Event.Type) + " " + dispatch.Event.Name,
		Data:      data,
	})
}
