package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/icoo-ai/icoo-ai/internal/tools"
)

const (
	defaultOpenAIResponsesBaseURL = "https://api.openai.com/v1"
	openAIResponsesPath           = "/responses"
)

type OpenAIResponsesConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

type OpenAIResponsesProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewOpenAIResponsesProvider(cfg OpenAIResponsesConfig) (*OpenAIResponsesProvider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if apiKey == "" {
		return nil, errors.New("openai responses provider requires API key")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIResponsesBaseURL
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid OpenAI base URL: %w", err)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &OpenAIResponsesProvider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      strings.TrimSpace(cfg.Model),
		httpClient: httpClient,
	}, nil
}

func (p *OpenAIResponsesProvider) Name() string {
	return "openai_responses"
}

func (p *OpenAIResponsesProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan CompletionEvent, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = p.model
	}
	if model == "" {
		return nil, errors.New("openai responses provider requires model")
	}

	body, err := json.Marshal(openAIResponsesRequest{
		Model:           model,
		Input:           openAIInput(req.Messages),
		Stream:          true,
		Tools:           openAITools(req.Tools),
		Temperature:     req.Options.Temperature,
		MaxOutputTokens: openAIMaxOutputTokens(req.Options.MaxOutputTokens),
		Reasoning:       openAIReasoning(req.Options.ReasoningEffort),
		Text:            openAITextConfig(req.Options.StructuredOutput),
		Metadata:        req.Options.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI responses request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+openAIResponsesPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create OpenAI responses request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai responses request failed: %w", sanitizeAPIKey(err, p.apiKey))
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, openAIErrorResponse(resp)
	}

	out := make(chan CompletionEvent)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		readOpenAIResponsesStream(ctx, resp.Body, out)
	}()
	return out, nil
}

type openAIResponsesRequest struct {
	Model           string            `json:"model"`
	Input           []map[string]any  `json:"input"`
	Stream          bool              `json:"stream"`
	Tools           []openAITool      `json:"tools,omitempty"`
	Temperature     *float64          `json:"temperature,omitempty"`
	MaxOutputTokens *int              `json:"max_output_tokens,omitempty"`
	Reasoning       map[string]string `json:"reasoning,omitempty"`
	Text            map[string]any    `json:"text,omitempty"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
}

type openAITool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func openAIInput(messages []Message) []map[string]any {
	input := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		if len(msg.ToolCalls) > 0 {
			for _, call := range msg.ToolCalls {
				item := map[string]any{
					"type":      "function_call",
					"call_id":   call.ID,
					"name":      call.Name,
					"arguments": string(call.Arguments),
					"status":    "completed",
				}
				if call.ItemID != "" {
					item["id"] = call.ItemID
				}
				input = append(input, item)
			}
			continue
		}
		if msg.Role == "tool" {
			item := map[string]any{
				"type":   "function_call_output",
				"output": msg.Content,
			}
			if callID, _ := msg.Metadata["tool_call_id"].(string); callID != "" {
				item["call_id"] = callID
			}
			input = append(input, item)
			continue
		}

		role := msg.Role
		if role == "" {
			role = "user"
		}
		input = append(input, map[string]any{
			"role":    role,
			"content": msg.Content,
		})
	}
	return input
}

func openAITools(defs []tools.ToolDefinition) []openAITool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]openAITool, 0, len(defs))
	for _, def := range defs {
		out = append(out, openAITool{
			Type:        "function",
			Name:        def.Name,
			Description: def.Description,
			Parameters:  def.InputSchema,
		})
	}
	return out
}

func openAIMaxOutputTokens(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func openAIReasoning(effort string) map[string]string {
	if strings.TrimSpace(effort) == "" {
		return nil
	}
	return map[string]string{"effort": effort}
}

func openAITextConfig(format map[string]any) map[string]any {
	if len(format) == 0 {
		return nil
	}
	return map[string]any{"format": format}
}

func readOpenAIResponsesStream(ctx context.Context, body io.Reader, out chan<- CompletionEvent) {
	reader := bufio.NewReader(body)
	var dataLines []string
	state := newOpenAIStreamState()
	for {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err != nil && !errors.Is(err, io.EOF) {
				emitCompletionEvent(ctx, out, CompletionEvent{Type: CompletionEventFailed, Error: err.Error()})
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) > 0 {
				if !handleOpenAIStreamData(ctx, strings.Join(dataLines, "\n"), state, out) {
					return
				}
				dataLines = dataLines[:0]
			}
			if err == io.EOF {
				return
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if err == io.EOF {
			if len(dataLines) > 0 {
				handleOpenAIStreamData(ctx, strings.Join(dataLines, "\n"), state, out)
			}
			return
		}
	}
}

type openAIStreamState struct {
	toolCalls map[string]*openAIStreamToolCall
	emitted   map[string]struct{}
}

type openAIStreamToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func newOpenAIStreamState() *openAIStreamState {
	return &openAIStreamState{
		toolCalls: make(map[string]*openAIStreamToolCall),
		emitted:   make(map[string]struct{}),
	}
}

type openAIStreamEvent struct {
	Type      string `json:"type"`
	Delta     string `json:"delta,omitempty"`
	Text      string `json:"text,omitempty"`
	ItemID    string `json:"item_id,omitempty"`
	Name      string `json:"name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Item      struct {
		Type      string `json:"type"`
		ID        string `json:"id,omitempty"`
		CallID    string `json:"call_id,omitempty"`
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"item,omitempty"`
	Response struct {
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code,omitempty"`
		} `json:"error,omitempty"`
	} `json:"response,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	} `json:"error,omitempty"`
}

func handleOpenAIStreamData(ctx context.Context, data string, state *openAIStreamState, out chan<- CompletionEvent) bool {
	if data == "[DONE]" {
		return emitCompletionEvent(ctx, out, CompletionEvent{Type: CompletionEventCompleted})
	}

	var event openAIStreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return emitCompletionEvent(ctx, out, CompletionEvent{Type: CompletionEventFailed, Error: "decode OpenAI stream event: " + err.Error()})
	}

	switch event.Type {
	case "response.output_text.delta":
		if event.Delta == "" {
			return true
		}
		return emitCompletionEvent(ctx, out, CompletionEvent{Type: CompletionEventMessageDelta, Delta: event.Delta})
	case "response.function_call_arguments.delta":
		if event.ItemID == "" {
			return true
		}
		call := state.toolCall(event.ItemID)
		call.Arguments.WriteString(event.Delta)
	case "response.function_call_arguments.done":
		return emitOpenAIToolCall(ctx, state, event.ItemID, event.CallID, event.Name, event.Arguments, out)
	case "response.output_item.done":
		if event.Item.Type != "function_call" {
			return true
		}
		return emitOpenAIToolCall(ctx, state, event.Item.ID, event.Item.CallID, event.Item.Name, event.Item.Arguments, out)
	case "response.completed":
		return emitCompletionEvent(ctx, out, CompletionEvent{Type: CompletionEventCompleted})
	case "response.failed", "response.incomplete":
		return emitCompletionEvent(ctx, out, CompletionEvent{Type: CompletionEventFailed, Error: openAIStreamError(event)})
	}
	return true
}

func (s *openAIStreamState) toolCall(itemID string) *openAIStreamToolCall {
	call := s.toolCalls[itemID]
	if call == nil {
		call = &openAIStreamToolCall{ID: itemID}
		s.toolCalls[itemID] = call
	}
	return call
}

func emitOpenAIToolCall(ctx context.Context, state *openAIStreamState, itemID, callID, name, arguments string, out chan<- CompletionEvent) bool {
	if itemID == "" && callID == "" {
		return true
	}
	call := state.toolCall(firstNonEmpty(itemID, callID))
	if callID != "" {
		call.ID = callID
	}
	if name != "" {
		call.Name = name
	}
	if arguments != "" {
		call.Arguments.Reset()
		call.Arguments.WriteString(arguments)
	}
	if call.Name == "" {
		return true
	}
	id := firstNonEmpty(call.ID, callID, itemID)
	if _, ok := state.emitted[id]; ok {
		return true
	}
	state.emitted[id] = struct{}{}
	return emitCompletionEvent(ctx, out, CompletionEvent{
		Type: CompletionEventToolCall,
		ToolCall: &tools.ToolCall{
			ID:        id,
			ItemID:    itemID,
			Name:      call.Name,
			Arguments: json.RawMessage(call.Arguments.String()),
		},
	})
}

func emitCompletionEvent(ctx context.Context, out chan<- CompletionEvent, event CompletionEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- event:
		return true
	}
}

func openAIStreamError(event openAIStreamEvent) string {
	if event.Error != nil && event.Error.Message != "" {
		return event.Error.Message
	}
	if event.Response.Error != nil && event.Response.Error.Message != "" {
		return event.Response.Error.Message
	}
	if event.Type == "response.incomplete" {
		return "OpenAI response incomplete"
	}
	return "OpenAI response failed"
}

func openAIErrorResponse(resp *http.Response) error {
	const maxErrorBody = 1 << 20
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	message := strings.TrimSpace(string(body))
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code,omitempty"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		message = parsed.Error.Message
	}
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("openai responses request failed with status %d: %s", resp.StatusCode, message)
}

func sanitizeAPIKey(err error, apiKey string) error {
	if err == nil || apiKey == "" {
		return err
	}
	if !strings.Contains(err.Error(), apiKey) {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), apiKey, "[redacted]"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
