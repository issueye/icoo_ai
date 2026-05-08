package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/policy"
)

type SearchClient interface {
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
}

type SearchRequest struct {
	Query      string
	MaxResults int
}

type SearchResponse struct {
	Provider  string
	FetchedAt time.Time
	Results   []SearchResult
}

type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet,omitempty"`
	URL     string `json:"url"`
	Time    string `json:"time,omitempty"`
}

type WebSearchOptions struct {
	Client      SearchClient
	Policy      policy.Policy
	AuditLogger audit.Logger
	Timeout     time.Duration
	MaxResults  int
	Now         func() time.Time
}

func NewWebSearchTool(opts WebSearchOptions) Tool {
	p := opts.Policy
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultWebSearchTimeout
	}
	maxResults := opts.MaxResults
	if maxResults <= 0 || maxResults > defaultWebSearchHardLimit {
		maxResults = defaultWebSearchMaxResults
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	client := opts.Client
	if client == nil {
		client = NewDuckDuckGoSearchClient(DuckDuckGoSearchOptions{Policy: p, Timeout: timeout})
	}
	return webSearchTool{
		client:      client,
		policy:      p,
		auditLogger: opts.AuditLogger,
		timeout:     timeout,
		maxResults:  maxResults,
		now:         now,
	}
}

type webSearchTool struct {
	client      SearchClient
	policy      policy.Policy
	auditLogger audit.Logger
	timeout     time.Duration
	maxResults  int
	now         func() time.Time
}

func (t webSearchTool) Name() string { return "web_search" }
func (t webSearchTool) Description() string {
	return "Search the web using DuckDuckGo and return sourced structured results."
}
func (t webSearchTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"max_results":{"type":"integer"}}}`)
}
func (t webSearchTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return toolError("invalid_input", "query is required", nil), nil
	}
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > t.maxResults {
		maxResults = t.maxResults
	}

	searchCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	resp, err := t.client.Search(searchCtx, SearchRequest{Query: req.Query, MaxResults: maxResults})
	if err != nil {
		_ = t.logSearch(searchCtx, req.Query, nil, false, err.Error())
		return toolError("search_failed", err.Error(), map[string]any{"query": req.Query}), nil
	}
	fetchedAt := resp.FetchedAt.UTC()
	if fetchedAt.IsZero() {
		fetchedAt = t.now().UTC()
	}
	if len(resp.Results) > maxResults {
		resp.Results = resp.Results[:maxResults]
	}
	for i := range resp.Results {
		resp.Results[i].Title = strings.TrimSpace(resp.Results[i].Title)
		resp.Results[i].Snippet = strings.TrimSpace(resp.Results[i].Snippet)
		resp.Results[i].URL = strings.TrimSpace(resp.Results[i].URL)
	}
	_ = t.logSearch(searchCtx, req.Query, resp.Results, true, "")

	lines := make([]string, 0, len(resp.Results))
	for i, result := range resp.Results {
		if result.Snippet != "" {
			lines = append(lines, fmt.Sprintf("%d. %s\n%s\n%s", i+1, result.Title, result.Snippet, result.URL))
		} else {
			lines = append(lines, fmt.Sprintf("%d. %s\n%s", i+1, result.Title, result.URL))
		}
	}
	provider := resp.Provider
	if provider == "" {
		provider = "duckduckgo"
	}
	data := map[string]any{
		"provider":   provider,
		"query":      req.Query,
		"fetched_at": fetchedAt.Format(time.RFC3339Nano),
		"results":    resp.Results,
	}
	sourceURLs := make([]string, 0, len(resp.Results))
	for _, result := range resp.Results {
		if result.URL != "" {
			sourceURLs = append(sourceURLs, result.URL)
		}
	}
	data["source_urls"] = sourceURLs
	return ToolResult{
		OK:      true,
		Content: strings.Join(lines, "\n\n"),
		Data:    data,
		Metadata: map[string]any{
			"provider":    provider,
			"source_urls": sourceURLs,
			"fetched_at":  fetchedAt.Format(time.RFC3339Nano),
		},
	}, nil
}

func (t webSearchTool) logSearch(ctx context.Context, query string, results []SearchResult, ok bool, errText string) error {
	if t.auditLogger == nil {
		return nil
	}
	sourceURLs := make([]string, 0, len(results))
	for _, result := range results {
		if result.URL != "" {
			sourceURLs = append(sourceURLs, result.URL)
		}
	}
	data := map[string]any{
		"query":        query,
		"source_urls":  sourceURLs,
		"result_count": len(results),
		"ok":           ok,
	}
	if errText != "" {
		data["error"] = errText
	}
	return t.auditLogger.Log(ctx, audit.Event{
		Type:      audit.EventNetworkAccess,
		Timestamp: t.now().UTC(),
		Summary:   "web_search " + query,
		Data:      data,
	})
}

type DuckDuckGoSearchOptions struct {
	BaseURL string
	Client  *http.Client
	Policy  policy.Policy
	Timeout time.Duration
	Now     func() time.Time
}

func NewDuckDuckGoSearchClient(opts DuckDuckGoSearchOptions) SearchClient {
	p := opts.Policy
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultWebSearchTimeout
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultDuckDuckGoLiteURL
	}
	client := opts.Client
	if client == nil {
		client = safeHTTPClient(p, defaultWebFetchRedirects)
	}
	return duckDuckGoSearchClient{
		baseURL: baseURL,
		client:  client,
		policy:  p,
		timeout: timeout,
		now:     now,
	}
}

type duckDuckGoSearchClient struct {
	baseURL string
	client  *http.Client
	policy  policy.Policy
	timeout time.Duration
	now     func() time.Time
}

func (c duckDuckGoSearchClient) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return SearchResponse{}, fmt.Errorf("query is required")
	}
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > defaultWebSearchHardLimit {
		maxResults = defaultWebSearchMaxResults
	}

	searchURL, err := buildDuckDuckGoURL(c.baseURL, query)
	if err != nil {
		return SearchResponse{}, err
	}
	if decision := authorizeNetwork(ctx, c.policy, searchURL, http.MethodGet); decision.Action == policy.DecisionBlock {
		return SearchResponse{}, fmt.Errorf("network policy blocked search provider: %s", decision.Reason)
	}

	searchCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(searchCtx, http.MethodGet, searchURL, nil)
	if err != nil {
		return SearchResponse{}, err
	}
	httpReq.Header.Set("User-Agent", "icoo-ai/0.1 web_search")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return SearchResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SearchResponse{}, fmt.Errorf("duckduckgo returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultWebFetchMaxBytes))
	if err != nil {
		return SearchResponse{}, err
	}
	results := parseDuckDuckGoLite(string(body), maxResults)
	return SearchResponse{
		Provider:  "duckduckgo",
		FetchedAt: c.now().UTC(),
		Results:   results,
	}, nil
}

func buildDuckDuckGoURL(baseURL, query string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("q", query)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

var (
	ddgResultLinkPattern = regexp.MustCompile(`(?is)<a[^>]+class=["'][^"']*result-link[^"']*["'][^>]+href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	ddgSnippetPattern    = regexp.MustCompile(`(?is)<td[^>]+class=["'][^"']*result-snippet[^"']*["'][^>]*>(.*?)</td>`)
)

func parseDuckDuckGoLite(body string, maxResults int) []SearchResult {
	linkMatches := ddgResultLinkPattern.FindAllStringSubmatchIndex(body, -1)
	snippetMatches := ddgSnippetPattern.FindAllStringSubmatchIndex(body, -1)
	results := make([]SearchResult, 0, minInt(maxResults, len(linkMatches)))
	for i, match := range linkMatches {
		if len(results) >= maxResults {
			break
		}
		rawURL := html.UnescapeString(body[match[2]:match[3]])
		rawTitle := body[match[4]:match[5]]
		result := SearchResult{
			Title: html.UnescapeString(stripHTMLTags(rawTitle)),
			URL:   normalizeDuckDuckGoResultURL(rawURL),
		}
		if i < len(snippetMatches) {
			sm := snippetMatches[i]
			result.Snippet = html.UnescapeString(strings.Join(strings.Fields(stripHTMLTags(body[sm[2]:sm[3]])), " "))
		}
		if result.Title != "" && result.URL != "" {
			results = append(results, result)
		}
	}
	return results
}

func normalizeDuckDuckGoResultURL(raw string) string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if strings.Contains(parsed.Host, "duckduckgo.com") {
		if uddg := parsed.Query().Get("uddg"); uddg != "" {
			if unescaped, err := url.QueryUnescape(uddg); err == nil {
				return unescaped
			}
			return uddg
		}
	}
	return raw
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
