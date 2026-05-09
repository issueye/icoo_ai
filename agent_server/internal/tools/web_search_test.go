package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebSearchUsesMockClientAndReturnsSources(t *testing.T) {
	client := &mockSearchClient{
		response: SearchResponse{
			Provider:  "mock",
			FetchedAt: fixedNow(),
			Results: []SearchResult{
				{Title: "Result One", Snippet: "summary", URL: "https://example.com/one"},
			},
		},
	}
	tool := NewWebSearchTool(WebSearchOptions{
		Client:     client,
		MaxResults: 5,
		Now:        fixedNow,
	})
	result := runTool(t, tool, map[string]any{"query": "golang", "max_results": 3})
	if !result.OK {
		t.Fatalf("search failed: %+v", result)
	}
	if client.calls != 1 || client.last.Query != "golang" || client.last.MaxResults != 3 {
		t.Fatalf("mock calls = %d last = %+v", client.calls, client.last)
	}
	if !strings.Contains(result.Content, "Result One") || !strings.Contains(result.Content, "https://example.com/one") {
		t.Fatalf("content = %q", result.Content)
	}
	urls, ok := result.Data["source_urls"].([]string)
	if !ok || len(urls) != 1 || urls[0] != "https://example.com/one" {
		t.Fatalf("source_urls = %#v", result.Data["source_urls"])
	}
	if result.Data["fetched_at"] != fixedNow().Format(time.RFC3339Nano) {
		t.Fatalf("fetched_at = %+v", result.Data["fetched_at"])
	}
}

func TestWebSearchDoesNotCallClientForInvalidQuery(t *testing.T) {
	client := &mockSearchClient{}
	tool := NewWebSearchTool(WebSearchOptions{Client: client})
	result := runTool(t, tool, map[string]any{"query": "   "})
	if result.OK || result.Data["code"] != "invalid_input" {
		t.Fatalf("result = %+v, want invalid_input", result)
	}
	if client.calls != 0 {
		t.Fatalf("client was called %d times", client.calls)
	}
}

func TestDuckDuckGoSearchClientParsesLiteHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "golang" {
			t.Fatalf("query = %q", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`
<html><body>
<a rel="nofollow" class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fa">Example &amp; A</a>
<td class="result-snippet">First <b>snippet</b></td>
<a rel="nofollow" class="result-link" href="https://example.com/b">Example B</a>
<td class="result-snippet">Second snippet</td>
</body></html>`))
	}))
	defer server.Close()

	client := NewDuckDuckGoSearchClient(DuckDuckGoSearchOptions{
		BaseURL: server.URL + "/lite/",
		Client:  server.Client(),
		Policy:  allowPolicy{},
		Now:     fixedNow,
	})
	resp, err := client.Search(context.Background(), SearchRequest{Query: "golang", MaxResults: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if resp.Provider != "duckduckgo" || !resp.FetchedAt.Equal(fixedNow()) {
		t.Fatalf("response metadata = %+v", resp)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("results = %+v", resp.Results)
	}
	if resp.Results[0].Title != "Example & A" || resp.Results[0].URL != "https://example.com/a" || resp.Results[0].Snippet != "First snippet" {
		t.Fatalf("result = %+v", resp.Results[0])
	}
}

func TestDuckDuckGoSearchClientUsesNetworkPolicy(t *testing.T) {
	client := NewDuckDuckGoSearchClient(DuckDuckGoSearchOptions{
		BaseURL: "https://example.com/lite/",
		Policy:  blockNetworkPolicy{},
	})
	_, err := client.Search(context.Background(), SearchRequest{Query: "golang"})
	if err == nil || !strings.Contains(err.Error(), "network policy blocked search provider") {
		t.Fatalf("err = %v", err)
	}
}

func TestDuckDuckGoSearchClientRetries429UntilSuccess(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			http.Error(w, "retry later", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`<a class="result-link" href="https://example.com/a">Example A</a><td class="result-snippet">Snippet</td>`))
	}))
	defer server.Close()

	client := NewDuckDuckGoSearchClient(DuckDuckGoSearchOptions{
		BaseURL: server.URL + "/lite/",
		Client:  server.Client(),
		Policy:  allowPolicy{},
		Now:     fixedNow,
	})
	resp, err := client.Search(context.Background(), SearchRequest{Query: "golang"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if requests != 3 || len(resp.Results) != 1 {
		t.Fatalf("requests=%d response=%+v", requests, resp)
	}
}

func TestDuckDuckGoSearchClientDoesNotRetry400(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewDuckDuckGoSearchClient(DuckDuckGoSearchOptions{
		BaseURL: server.URL + "/lite/",
		Client:  server.Client(),
		Policy:  allowPolicy{},
	})
	_, err := client.Search(context.Background(), SearchRequest{Query: "golang"})
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("err = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

type mockSearchClient struct {
	response SearchResponse
	err      error
	calls    int
	last     SearchRequest
}

func (m *mockSearchClient) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	m.calls++
	m.last = req
	if m.err != nil {
		return SearchResponse{}, m.err
	}
	return m.response, nil
}
