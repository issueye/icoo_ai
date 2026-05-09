package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/api"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
)

func TestCreateAndListSessions(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())

	session := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{
		"title":   "API slice",
		"cwd":     "E:/code/issueye/icoo_ai",
		"agentId": "icoo-ai-acp",
	})
	if session.ID == "" {
		t.Fatal("expected session id")
	}
	if session.Title != "API slice" {
		t.Fatalf("expected title %q, got %q", "API slice", session.Title)
	}

	sessions := doJSON[[]service.Session](t, router, http.MethodGet, "/v1/sessions", nil)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != session.ID {
		t.Fatalf("expected listed session %q, got %q", session.ID, sessions[0].ID)
	}
}

func TestPromptCreatesMessagesAndApproval(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())
	session := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{
		"title": "Prompt test",
	})

	prompt := doJSON[service.PromptResponse](t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/prompt", map[string]string{
		"content": "hello",
	})
	if prompt.Run.Status != "completed" {
		t.Fatalf("expected completed run, got %q", prompt.Run.Status)
	}
	if len(prompt.Messages) != 2 {
		t.Fatalf("expected 2 prompt messages, got %d", len(prompt.Messages))
	}
	if prompt.Messages[0].Role != "user" || prompt.Messages[1].Role != "assistant" {
		t.Fatalf("expected user and assistant messages, got %#v", prompt.Messages)
	}
	if prompt.Approval == nil || prompt.Approval.Status != "pending" {
		t.Fatalf("expected pending approval, got %#v", prompt.Approval)
	}

	messages := doJSON[[]service.Message](t, router, http.MethodGet, "/v1/sessions/"+session.ID+"/messages", nil)
	if len(messages) != 2 {
		t.Fatalf("expected 2 stored messages, got %d", len(messages))
	}
}

func TestCancelCreatesCancelledRun(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())
	session := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{
		"title": "Cancel test",
	})

	run := doJSON[service.Run](t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/cancel", nil)
	if run.Status != "cancelled" {
		t.Fatalf("expected cancelled run, got %q", run.Status)
	}
	if run.SessionID != session.ID {
		t.Fatalf("expected run session %q, got %q", session.ID, run.SessionID)
	}

	runs := doJSON[[]service.Run](t, router, http.MethodGet, "/v1/runs", nil)
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("expected listed cancelled run %#v, got %#v", run, runs)
	}
}

func TestApprovalDecision(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())
	session := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{
		"title": "Approval test",
	})
	prompt := doJSON[service.PromptResponse](t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/prompt", map[string]string{
		"content": "needs a tool",
	})
	if prompt.Approval == nil {
		t.Fatal("expected prompt approval")
	}

	approval := doJSON[service.Approval](t, router, http.MethodPost, "/v1/approvals/"+prompt.Approval.ID+"/decision", map[string]string{
		"decision": "approved",
	})
	if approval.Status != "approved" {
		t.Fatalf("expected approved status, got %q", approval.Status)
	}
	if approval.DecidedAt == nil {
		t.Fatal("expected decided timestamp")
	}

	approvals := doJSON[[]service.Approval](t, router, http.MethodGet, "/v1/approvals", nil)
	if len(approvals) != 1 || approvals[0].Status != "approved" {
		t.Fatalf("expected listed approved approval, got %#v", approvals)
	}
}

func TestApprovalDecisionAfterCancelReturnsStructuredError(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())
	session := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{
		"title": "Approval cancel test",
	})
	prompt := doJSON[service.PromptResponse](t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/prompt", map[string]string{
		"content": "needs approval",
	})
	if prompt.Approval == nil {
		t.Fatal("expected prompt approval")
	}

	doJSON[service.Run](t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/cancel", nil)

	reqBody, err := json.Marshal(map[string]string{"decision": "approved"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals/"+prompt.Approval.ID+"/decision", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	var response api.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Code != "invalid_decision" || response.Message == "" {
		t.Fatalf("expected structured invalid_decision error, got %#v", response)
	}
}

func TestMissingSessionReturnsJSONError(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/missing/messages", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	var response api.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Code != "session_not_found" || response.Message == "" {
		t.Fatalf("expected session_not_found error, got %#v", response)
	}
}

func TestConcurrentSessionPromptCancelAndApprovalIsolation(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())
	sessionA := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{"title": "A"})
	sessionB := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{"title": "B"})

	var (
		promptA service.PromptResponse
		promptB service.PromptResponse
		errA    error
		errB    error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		promptA, errA = doJSONNoFail[service.PromptResponse](router, http.MethodPost, "/v1/sessions/"+sessionA.ID+"/prompt", map[string]string{"content": "hello from A"})
	}()
	go func() {
		defer wg.Done()
		promptB, errB = doJSONNoFail[service.PromptResponse](router, http.MethodPost, "/v1/sessions/"+sessionB.ID+"/prompt", map[string]string{"content": "hello from B"})
	}()
	wg.Wait()
	if errA != nil {
		t.Fatalf("prompt A failed: %v", errA)
	}
	if errB != nil {
		t.Fatalf("prompt B failed: %v", errB)
	}

	if promptA.Approval == nil || promptB.Approval == nil {
		t.Fatal("expected approvals for both sessions")
	}

	var (
		cancelRun service.Run
		approved  service.Approval
		cancelErr error
		decideErr error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		cancelRun, cancelErr = doJSONNoFail[service.Run](router, http.MethodPost, "/v1/sessions/"+sessionA.ID+"/cancel", nil)
	}()
	go func() {
		defer wg.Done()
		approved, decideErr = doJSONNoFail[service.Approval](router, http.MethodPost, "/v1/approvals/"+promptB.Approval.ID+"/decision", map[string]string{"decision": "approved"})
	}()
	wg.Wait()
	if cancelErr != nil {
		t.Fatalf("cancel A failed: %v", cancelErr)
	}
	if decideErr != nil {
		t.Fatalf("decide B failed: %v", decideErr)
	}

	if cancelRun.SessionID != sessionA.ID || cancelRun.Status != "cancelled" {
		t.Fatalf("expected cancel run for session A, got %#v", cancelRun)
	}
	if approved.SessionID != sessionB.ID || approved.Status != "approved" {
		t.Fatalf("expected approved decision for session B, got %#v", approved)
	}

	approvals := doJSON[[]service.Approval](t, router, http.MethodGet, "/v1/approvals", nil)
	byID := make(map[string]service.Approval, len(approvals))
	for _, approval := range approvals {
		byID[approval.ID] = approval
	}
	if got := byID[promptA.Approval.ID]; got.SessionID != sessionA.ID || got.Status != "expired" || got.Decision != "rejected" {
		t.Fatalf("expected session A approval expired/rejected, got %#v", got)
	}
	if got := byID[promptB.Approval.ID]; got.SessionID != sessionB.ID || got.Status != "approved" || got.Decision != "approved" {
		t.Fatalf("expected session B approval approved, got %#v", got)
	}

	messagesA := doJSON[[]service.Message](t, router, http.MethodGet, "/v1/sessions/"+sessionA.ID+"/messages", nil)
	messagesB := doJSON[[]service.Message](t, router, http.MethodGet, "/v1/sessions/"+sessionB.ID+"/messages", nil)
	for _, msg := range messagesA {
		if msg.SessionID != sessionA.ID {
			t.Fatalf("session A message crossed sessions: %#v", msg)
		}
	}
	for _, msg := range messagesB {
		if msg.SessionID != sessionB.ID {
			t.Fatalf("session B message crossed sessions: %#v", msg)
		}
	}
}

func doJSON[T any](t *testing.T, handler http.Handler, method, path string, body any) T {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s returned %d: %s", method, path, rec.Code, rec.Body.String())
	}

	var response T
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func doJSONNoFail[T any](handler http.Handler, method, path string, body any) (T, error) {
	var zero T
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return zero, fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		return zero, fmt.Errorf("%s %s returned %d: %s", method, path, rec.Code, rec.Body.String())
	}

	var response T
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return response, nil
}
