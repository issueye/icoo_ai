package netutil

import (
	"net/http"
	"testing"
)

func TestProxyFuncUsesConfiguredProxyAndNoProxy(t *testing.T) {
	proxy, err := ProxyFunc(ProxyConfig{
		HTTPProxy:  "http://proxy.local:8080",
		HTTPSProxy: "http://secure-proxy.local:8080",
		NoProxy:    "localhost,.internal",
	})
	if err != nil {
		t.Fatalf("ProxyFunc() error = %v", err)
	}

	httpReq, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	httpProxy, err := proxy(httpReq)
	if err != nil {
		t.Fatalf("http proxy error = %v", err)
	}
	if httpProxy.String() != "http://proxy.local:8080" {
		t.Fatalf("http proxy = %s", httpProxy)
	}

	httpsReq, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	httpsProxy, err := proxy(httpsReq)
	if err != nil {
		t.Fatalf("https proxy error = %v", err)
	}
	if httpsProxy.String() != "http://secure-proxy.local:8080" {
		t.Fatalf("https proxy = %s", httpsProxy)
	}

	noProxyReq, _ := http.NewRequest(http.MethodGet, "https://api.internal", nil)
	got, err := proxy(noProxyReq)
	if err != nil {
		t.Fatalf("no proxy error = %v", err)
	}
	if got != nil {
		t.Fatalf("no_proxy request used proxy %s", got)
	}
}
