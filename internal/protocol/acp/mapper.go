package acp

import (
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/internal/agent"
)

func mapNewSessionRequest(req sdk.NewSessionRequest) agent.NewSessionRequest {
	metadata := cloneMeta(req.Meta)
	if len(req.AdditionalDirectories) > 0 {
		metadata["additional_directories"] = append([]string(nil), req.AdditionalDirectories...)
	}
	if len(req.McpServers) > 0 {
		metadata["mcp_servers"] = req.McpServers
	}
	return agent.NewSessionRequest{
		CWD:      req.Cwd,
		Metadata: metadataOrNil(metadata),
	}
}

func mapPromptRequest(req sdk.PromptRequest) agent.PromptRequest {
	metadata := cloneMeta(req.Meta)
	if req.MessageId != nil {
		metadata["message_id"] = *req.MessageId
	}
	return agent.PromptRequest{
		SessionID: string(req.SessionId),
		Prompt:    promptText(req.Prompt),
		Metadata:  metadataOrNil(metadata),
	}
}

func promptText(blocks []sdk.ContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch {
		case block.Text != nil:
			parts = append(parts, block.Text.Text)
		case block.ResourceLink != nil:
			parts = append(parts, fmt.Sprintf("[%s](%s)", block.ResourceLink.Name, block.ResourceLink.Uri))
		case block.Resource != nil:
			parts = append(parts, resourceText(block.Resource.Resource))
		default:
			if raw, err := json.Marshal(block); err == nil {
				parts = append(parts, string(raw))
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func resourceText(resource sdk.EmbeddedResourceResource) string {
	switch {
	case resource.TextResourceContents != nil:
		return resource.TextResourceContents.Text
	case resource.BlobResourceContents != nil:
		return resource.BlobResourceContents.Blob
	default:
		if raw, err := json.Marshal(resource); err == nil {
			return string(raw)
		}
		return ""
	}
}

func mapSessionEvent(event agent.Event) (sdk.SessionUpdate, bool) {
	switch event.Type {
	case agent.EventRunStarted:
		return sdk.UpdateAgentThoughtText("Started."), true
	case agent.EventMessageDelta:
		if event.Content == "" {
			return sdk.SessionUpdate{}, false
		}
		return sdk.UpdateAgentMessageText(event.Content), true
	case agent.EventToolCallStarted:
		return mapToolCallStarted(event), true
	case agent.EventToolCallCompleted:
		return mapToolCallCompleted(event), true
	case agent.EventApprovalRequested:
		return sdk.UpdateAgentThoughtText(contentOrError(event, "Approval requested.")), true
	case agent.EventPlanUpdated:
		update, ok := mapPlanUpdate(event)
		return update, ok
	case agent.EventRunFailed:
		return sdk.UpdateAgentThoughtText(contentOrError(event, "Run failed.")), true
	case agent.EventRunCancelled:
		return sdk.UpdateAgentThoughtText(contentOrError(event, "Run cancelled.")), true
	case agent.EventRunCompleted:
		return sdk.UpdateAgentThoughtText("Completed."), true
	default:
		return sdk.SessionUpdate{}, false
	}
}

func mapToolCallStarted(event agent.Event) sdk.SessionUpdate {
	id := stringFromData(event.Data, "id", "tool")
	name := stringFromData(event.Data, "name", "tool")
	opts := []sdk.ToolCallStartOpt{
		sdk.WithStartStatus(sdk.ToolCallStatusPending),
		sdk.WithStartKind(toolKind(name)),
	}
	if input, ok := event.Data["input"]; ok {
		opts = append(opts, sdk.WithStartRawInput(input))
	}
	if locations := locationsFromData(event.Data); len(locations) > 0 {
		opts = append(opts, sdk.WithStartLocations(locations))
	}
	return sdk.StartToolCall(sdk.ToolCallId(id), name, opts...)
}

func mapToolCallCompleted(event agent.Event) sdk.SessionUpdate {
	id := stringFromData(event.Data, "id", "tool")
	status := sdk.ToolCallStatusCompleted
	if event.Error != "" {
		status = sdk.ToolCallStatusFailed
	}
	opts := []sdk.ToolCallUpdateOpt{
		sdk.WithUpdateStatus(status),
	}
	if event.Content != "" {
		opts = append(opts, sdk.WithUpdateContent([]sdk.ToolCallContent{
			sdk.ToolContent(sdk.TextBlock(event.Content)),
		}))
	}
	if result, ok := event.Data["result"]; ok {
		opts = append(opts, sdk.WithUpdateRawOutput(result))
	} else if event.Error != "" {
		opts = append(opts, sdk.WithUpdateRawOutput(map[string]any{"error": event.Error}))
	}
	return sdk.UpdateToolCall(sdk.ToolCallId(id), opts...)
}

func mapPlanUpdate(event agent.Event) (sdk.SessionUpdate, bool) {
	items, ok := event.Data["entries"].([]any)
	if !ok {
		items, ok = event.Data["plan"].([]any)
	}
	if !ok || len(items) == 0 {
		return sdk.SessionUpdate{}, false
	}
	entries := make([]sdk.PlanEntry, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, _ := m["content"].(string)
		if content == "" {
			content, _ = m["step"].(string)
		}
		if content == "" {
			continue
		}
		entries = append(entries, sdk.PlanEntry{
			Content:  content,
			Status:   planStatus(fmt.Sprint(m["status"])),
			Priority: planPriority(fmt.Sprint(m["priority"])),
		})
	}
	if len(entries) == 0 {
		return sdk.SessionUpdate{}, false
	}
	return sdk.UpdatePlan(entries...), true
}

func stopReasonForEvent(event agent.Event) (sdk.StopReason, bool) {
	switch event.Type {
	case agent.EventRunCompleted:
		return sdk.StopReasonEndTurn, true
	case agent.EventRunCancelled:
		return sdk.StopReasonCancelled, true
	case agent.EventRunFailed:
		return sdk.StopReasonRefusal, true
	default:
		return "", false
	}
}

func contentOrError(event agent.Event, fallback string) string {
	if event.Content != "" {
		return event.Content
	}
	if event.Error != "" {
		return event.Error
	}
	return fallback
}

func stringFromData(data map[string]any, key string, fallback string) string {
	if data == nil {
		return fallback
	}
	switch v := data[key].(type) {
	case string:
		if v != "" {
			return v
		}
	case fmt.Stringer:
		return v.String()
	}
	return fallback
}

func locationsFromData(data map[string]any) []sdk.ToolCallLocation {
	if data == nil {
		return nil
	}
	path := stringFromData(data, "path", "")
	if path == "" {
		path = stringFromData(data, "file", "")
	}
	if path == "" {
		return nil
	}
	return []sdk.ToolCallLocation{{Path: path}}
}

func toolKind(name string) sdk.ToolKind {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "read"):
		return sdk.ToolKindRead
	case strings.Contains(n, "write"), strings.Contains(n, "edit"), strings.Contains(n, "patch"):
		return sdk.ToolKindEdit
	case strings.Contains(n, "delete"), strings.Contains(n, "remove"):
		return sdk.ToolKindDelete
	case strings.Contains(n, "search"), strings.Contains(n, "grep"):
		return sdk.ToolKindSearch
	case strings.Contains(n, "shell"), strings.Contains(n, "exec"), strings.Contains(n, "run"):
		return sdk.ToolKindExecute
	case strings.Contains(n, "fetch"), strings.Contains(n, "web"):
		return sdk.ToolKindFetch
	default:
		return sdk.ToolKindOther
	}
}

func planStatus(status string) sdk.PlanEntryStatus {
	switch status {
	case string(sdk.PlanEntryStatusInProgress), "in-progress", "running":
		return sdk.PlanEntryStatusInProgress
	case string(sdk.PlanEntryStatusCompleted), "done":
		return sdk.PlanEntryStatusCompleted
	default:
		return sdk.PlanEntryStatusPending
	}
}

func planPriority(priority string) sdk.PlanEntryPriority {
	switch priority {
	case string(sdk.PlanEntryPriorityHigh):
		return sdk.PlanEntryPriorityHigh
	case string(sdk.PlanEntryPriorityLow):
		return sdk.PlanEntryPriorityLow
	default:
		return sdk.PlanEntryPriorityMedium
	}
}

func cloneMeta(meta map[string]any) map[string]any {
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func metadataOrNil(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}
