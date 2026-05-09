package tools

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/netutil"
	"github.com/icoo-ai/icoo-ai/internal/policy"
)

const (
	defaultWebFetchMaxBytes    int64 = 1024 * 1024
	defaultWebFetchTimeout           = 15 * time.Second
	defaultWebFetchRedirects         = 3
	defaultWebSearchTimeout          = 15 * time.Second
	defaultWebSearchMaxResults       = 10
	defaultWebSearchHardLimit        = 20
	defaultDuckDuckGoLiteURL         = "https://lite.duckduckgo.com/lite/"
)

type WebFetchOptions struct {
	MaxBytes     int64
	Timeout      time.Duration
	MaxRedirects int
	Policy       policy.Policy
	Proxy        netutil.ProxyConfig
	AuditLogger  audit.Logger
	Now          func() time.Time
}

func NewWebFetchTool(opts WebFetchOptions) Tool {
	p := opts.Policy
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultWebFetchMaxBytes
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultWebFetchTimeout
	}
	maxRedirects := opts.MaxRedirects
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	if opts.MaxRedirects == 0 {
		maxRedirects = defaultWebFetchRedirects
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return webFetchTool{
		policy:       p,
		maxBytes:     maxBytes,
		timeout:      timeout,
		maxRedirects: maxRedirects,
		proxy:        opts.Proxy,
		auditLogger:  opts.AuditLogger,
		now:          now,
	}
}

type webFetchTool struct {
	policy       policy.Policy
	maxBytes     int64
	timeout      time.Duration
	maxRedirects int
	proxy        netutil.ProxyConfig
	auditLogger  audit.Logger
	now          func() time.Time
}

func (t webFetchTool) Name() string { return "web_fetch" }
func (t webFetchTool) Description() string {
	return "Fetch an HTTP(S) URL with SSRF protection, timeout, redirect, and size limits."
}
func (t webFetchTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["url"],"properties":{"url":{"type":"string"},"max_bytes":{"type":"integer"},"timeout_seconds":{"type":"number"}}}`)
}
func (t webFetchTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		URL            string  `json:"url"`
		MaxBytes       int64   `json:"max_bytes"`
		TimeoutSeconds float64 `json:"timeout_seconds"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	if strings.TrimSpace(req.URL) == "" {
		return toolError("invalid_input", "url is required", nil), nil
	}

	maxBytes := t.maxBytes
	if req.MaxBytes > 0 && req.MaxBytes < maxBytes {
		maxBytes = req.MaxBytes
	}
	timeout := t.timeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds * float64(time.Second))
		if timeout > t.timeout {
			timeout = t.timeout
		}
	}

	startedAt := t.now().UTC()
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if decision := authorizeNetwork(fetchCtx, t.policy, req.URL, http.MethodGet); decision.Action == policy.DecisionBlock {
		_ = t.logNetwork(fetchCtx, req.URL, "", startedAt, 0, "", false, decision.Reason, 0, decision)
		return toolError("policy_blocked", decision.Reason, decision.Details), nil
	}

	client, err := safeHTTPClient(t.policy, t.maxRedirects, t.proxy)
	if err != nil {
		return toolError("proxy_config_error", err.Error(), nil), nil
	}
	httpReq, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, req.URL, nil)
	if err != nil {
		return toolError("invalid_url", err.Error(), map[string]any{"url": req.URL}), nil
	}
	httpReq.Header.Set("User-Agent", "icoo-ai/0.1 web_fetch")

	resp, attempts, err := retryHTTP(fetchCtx, 0, func() (*http.Response, error) {
		reqCopy := httpReq.Clone(fetchCtx)
		return client.Do(reqCopy)
	})
	if err != nil {
		_ = t.logNetwork(fetchCtx, req.URL, "", startedAt, 0, "", false, err.Error(), attempts, policy.Decision{})
		if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
			return toolError("timeout", "web fetch timed out", map[string]any{"url": req.URL}), nil
		}
		return toolError("fetch_failed", err.Error(), map[string]any{"url": req.URL}), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = t.logNetwork(fetchCtx, req.URL, resp.Request.URL.String(), startedAt, resp.StatusCode, resp.Header.Get("Content-Type"), false, fmt.Sprintf("unexpected status %d", resp.StatusCode), attempts, policy.Decision{})
		return toolError("fetch_failed", fmt.Sprintf("unexpected status %d", resp.StatusCode), map[string]any{
			"url":            req.URL,
			"status_code":    resp.StatusCode,
			"retry_attempts": attempts,
		}), nil
	}

	body, truncated, err := readLimited(resp.Body, maxBytes)
	if err != nil {
		_ = t.logNetwork(fetchCtx, req.URL, resp.Request.URL.String(), startedAt, resp.StatusCode, resp.Header.Get("Content-Type"), false, err.Error(), attempts, policy.Decision{})
		return toolError("read_failed", err.Error(), map[string]any{"url": req.URL}), nil
	}

	fetchedAt := t.now().UTC()
	contentType := resp.Header.Get("Content-Type")
	finalURL := resp.Request.URL.String()
	data := map[string]any{
		"source_url":     req.URL,
		"final_url":      finalURL,
		"fetched_at":     fetchedAt.Format(time.RFC3339Nano),
		"status_code":    resp.StatusCode,
		"content_type":   contentType,
		"bytes":          len(body),
		"truncated":      truncated,
		"retry_attempts": attempts,
	}
	if title := extractHTMLTitle(string(body), contentType); title != "" {
		data["title"] = title
	}
	_ = t.logNetwork(fetchCtx, req.URL, finalURL, fetchedAt, resp.StatusCode, contentType, true, "", attempts, policy.Decision{})

	return ToolResult{
		OK:      true,
		Content: string(body),
		Data:    data,
		Metadata: map[string]any{
			"source_url":   req.URL,
			"final_url":    finalURL,
			"fetched_at":   fetchedAt.Format(time.RFC3339Nano),
			"status_code":  resp.StatusCode,
			"content_type": contentType,
		},
	}, nil
}

func (t webFetchTool) logNetwork(ctx context.Context, sourceURL, finalURL string, at time.Time, statusCode int, contentType string, ok bool, errText string, attempts int, decision policy.Decision) error {
	if t.auditLogger == nil {
		return nil
	}
	data := map[string]any{
		"url":            sourceURL,
		"final_url":      finalURL,
		"fetched_at":     at.UTC().Format(time.RFC3339Nano),
		"status_code":    statusCode,
		"content_type":   contentType,
		"ok":             ok,
		"retry_attempts": attempts,
	}
	if errText != "" {
		data["error"] = errText
	}
	if decision.Action != "" {
		data["policy_decision"] = decision
	}
	return t.auditLogger.Log(ctx, audit.Event{
		Type:      audit.EventNetworkAccess,
		Timestamp: at.UTC(),
		Summary:   "web_fetch " + sourceURL,
		Data:      data,
	})
}

func safeHTTPClient(p policy.Policy, maxRedirects int, proxyCfg netutil.ProxyConfig) (*http.Client, error) {
	proxy, err := netutil.ProxyFunc(proxyCfg)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy: proxy,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolveHost(ctx, host)
			if err != nil {
				return nil, err
			}
			decision := p.EvaluateNetwork(policy.NetworkRequest{
				URL:         "https://" + net.JoinHostPort(host, port),
				Method:      http.MethodGet,
				ResolvedIPs: ips,
			})
			if decision.Action == policy.DecisionBlock {
				return nil, fmt.Errorf("network policy blocked %s: %s", host, decision.Reason)
			}
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			decision := authorizeNetwork(req.Context(), p, req.URL.String(), req.Method)
			if decision.Action == policy.DecisionBlock {
				return fmt.Errorf("network policy blocked redirect to %s: %s", req.URL.String(), decision.Reason)
			}
			return nil
		},
	}, nil
}

func authorizeNetwork(ctx context.Context, p policy.Policy, rawURL, method string) policy.Decision {
	resolved := []string{}
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Hostname() != "" {
		if ips, err := resolveHost(ctx, parsed.Hostname()); err == nil {
			resolved = ips
		}
	}
	return p.EvaluateNetwork(policy.NetworkRequest{URL: rawURL, Method: method, ResolvedIPs: resolved})
}

func resolveHost(ctx context.Context, host string) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []string{ip.String()}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP.String())
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host %q resolved to no addresses", host)
	}
	return ips, nil
}

func readLimited(r io.Reader, maxBytes int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > maxBytes {
		return data[:maxBytes], true, nil
	}
	return data, false, nil
}

var titlePattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func extractHTMLTitle(body, contentType string) string {
	if !strings.Contains(strings.ToLower(contentType), "html") {
		return ""
	}
	match := titlePattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(stripHTMLTags(match[1])))
}

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

func stripHTMLTags(value string) string {
	return strings.TrimSpace(htmlTagPattern.ReplaceAllString(value, ""))
}
