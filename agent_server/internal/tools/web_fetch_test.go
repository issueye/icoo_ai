package tools

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/netutil"
	"github.com/icoo-ai/icoo-ai/internal/policy"
)

func TestWebFetchReturnsMetadataAndContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("<html><head><title>Example &amp; Title</title></head><body>hello</body></html>"))
	}))
	defer server.Close()

	tool := NewWebFetchTool(WebFetchOptions{
		Policy: allowPolicy{},
		Now:    fixedNow,
	})
	result := runTool(t, tool, map[string]any{"url": server.URL})
	if !result.OK {
		t.Fatalf("fetch failed: %+v", result)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Fatalf("content = %q, want body", result.Content)
	}
	if result.Data["source_url"] != server.URL || result.Data["final_url"] != server.URL {
		t.Fatalf("missing URLs: %+v", result.Data)
	}
	if result.Data["status_code"] != http.StatusAccepted {
		t.Fatalf("status_code = %+v", result.Data["status_code"])
	}
	if result.Data["content_type"] != "text/html; charset=utf-8" {
		t.Fatalf("content_type = %+v", result.Data["content_type"])
	}
	if result.Data["fetched_at"] != fixedNow().Format(time.RFC3339Nano) {
		t.Fatalf("fetched_at = %+v", result.Data["fetched_at"])
	}
	if result.Data["title"] != "Example & Title" {
		t.Fatalf("title = %+v", result.Data["title"])
	}
}

func TestWebFetchTruncatesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("abcdef"))
	}))
	defer server.Close()

	tool := NewWebFetchTool(WebFetchOptions{Policy: allowPolicy{}, MaxBytes: 4})
	result := runTool(t, tool, map[string]any{"url": server.URL})
	if !result.OK {
		t.Fatalf("fetch failed: %+v", result)
	}
	if result.Content != "abcd" || result.Data["truncated"] != true {
		t.Fatalf("result = %+v", result)
	}
}

func TestWebFetchBlocksUnsafeURLByDefaultPolicy(t *testing.T) {
	tool := NewWebFetchTool(WebFetchOptions{})
	result := runTool(t, tool, map[string]any{"url": "http://127.0.0.1:8080"})
	if result.OK || result.Data["code"] != "policy_blocked" {
		t.Fatalf("result = %+v, want policy_blocked", result)
	}
}

func TestWebFetchLimitsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer server.Close()

	tool := NewWebFetchTool(WebFetchOptions{Policy: allowPolicy{}, MaxRedirects: 1})
	result := runTool(t, tool, map[string]any{"url": server.URL})
	if result.OK || result.Data["code"] != "fetch_failed" {
		t.Fatalf("result = %+v, want fetch_failed", result)
	}
	if !strings.Contains(result.Error, "stopped after 1 redirects") {
		t.Fatalf("error = %q", result.Error)
	}
}

func TestWebFetchUsesConfiguredHTTPProxy(t *testing.T) {
	requests := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.String() != "http://example.com/resource" {
			t.Fatalf("proxy URL = %s", r.URL.String())
		}
		_, _ = w.Write([]byte("proxied"))
	}))
	defer proxy.Close()

	tool := NewWebFetchTool(WebFetchOptions{
		Policy: allowProxyPolicy{},
		Proxy:  netutil.ProxyConfig{HTTPProxy: proxy.URL},
	})
	result := runTool(t, tool, map[string]any{"url": "http://example.com/resource"})
	if !result.OK || result.Content != "proxied" {
		t.Fatalf("result = %+v", result)
	}
	if requests != 1 {
		t.Fatalf("proxy requests = %d, want 1", requests)
	}
}

func TestWebFetchRetries429UntilSuccess(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			http.Error(w, "retry later", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	tool := NewWebFetchTool(WebFetchOptions{Policy: allowPolicy{}})
	result := runTool(t, tool, map[string]any{"url": server.URL})
	if !result.OK || result.Content != "ok" {
		t.Fatalf("result = %+v", result)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestWebFetchDoesNotRetry400(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	tool := NewWebFetchTool(WebFetchOptions{Policy: allowPolicy{}})
	result := runTool(t, tool, map[string]any{"url": server.URL})
	if result.OK || !strings.Contains(result.Error, "unexpected status 400") {
		t.Fatalf("result = %+v", result)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

type allowPolicy struct{}

func (allowPolicy) EvaluateCommand(policy.CommandRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (allowPolicy) EvaluatePath(policy.PathRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (allowPolicy) EvaluateNetwork(policy.NetworkRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (allowPolicy) EvaluateMCP(policy.MCPRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}

type blockNetworkPolicy struct{}

func (blockNetworkPolicy) EvaluateCommand(policy.CommandRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (blockNetworkPolicy) EvaluatePath(policy.PathRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (blockNetworkPolicy) EvaluateNetwork(req policy.NetworkRequest) policy.Decision {
	return policy.Decision{
		Action: policy.DecisionBlock,
		Risk:   policy.RiskLevelCritical,
		Reason: "blocked " + req.URL,
		Details: map[string]any{
			"url": req.URL,
		},
	}
}
func (blockNetworkPolicy) EvaluateMCP(policy.MCPRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 8, 1, 2, 3, 4, time.UTC)
}

type allowProxyPolicy struct{}

func (allowProxyPolicy) EvaluateCommand(policy.CommandRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (allowProxyPolicy) EvaluatePath(policy.PathRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (allowProxyPolicy) EvaluateNetwork(req policy.NetworkRequest) policy.Decision {
	if len(req.ResolvedIPs) == 0 {
		return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
	}
	for _, ipText := range req.ResolvedIPs {
		ip := net.ParseIP(ipText)
		if ip != nil && ip.IsLoopback() {
			return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
		}
	}
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (allowProxyPolicy) EvaluateMCP(policy.MCPRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
