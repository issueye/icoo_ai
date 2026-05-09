package netutil

import (
	"net/http"
	"net/url"
	"strings"
)

type ProxyConfig struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
}

func ProxyFunc(cfg ProxyConfig) (func(*http.Request) (*url.URL, error), error) {
	cfg.HTTPProxy = strings.TrimSpace(cfg.HTTPProxy)
	cfg.HTTPSProxy = strings.TrimSpace(cfg.HTTPSProxy)
	cfg.NoProxy = strings.TrimSpace(cfg.NoProxy)
	if cfg.HTTPProxy == "" && cfg.HTTPSProxy == "" && cfg.NoProxy == "" {
		return http.ProxyFromEnvironment, nil
	}

	env := &httpproxyConfig{
		HTTPProxy:  cfg.HTTPProxy,
		HTTPSProxy: cfg.HTTPSProxy,
		NoProxy:    cfg.NoProxy,
	}
	return env.proxyFunc()
}

type httpproxyConfig struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
}

func (c *httpproxyConfig) proxyFunc() (func(*http.Request) (*url.URL, error), error) {
	httpProxy, err := parseProxyURL(c.HTTPProxy)
	if err != nil {
		return nil, err
	}
	httpsProxy, err := parseProxyURL(c.HTTPSProxy)
	if err != nil {
		return nil, err
	}
	noProxy := parseNoProxy(c.NoProxy)
	return func(req *http.Request) (*url.URL, error) {
		if req == nil || req.URL == nil {
			return nil, nil
		}
		if noProxy.match(req.URL.Hostname()) {
			return nil, nil
		}
		if req.URL.Scheme == "https" {
			return httpsProxy, nil
		}
		if req.URL.Scheme == "http" {
			return httpProxy, nil
		}
		return nil, nil
	}, nil
}

func parseProxyURL(value string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	return url.Parse(value)
}

type noProxyRules []string

func parseNoProxy(value string) noProxyRules {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	rules := make(noProxyRules, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			rules = append(rules, part)
		}
	}
	return rules
}

func (rules noProxyRules) match(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, rule := range rules {
		if rule == "*" || rule == host {
			return true
		}
		if strings.HasPrefix(rule, ".") && strings.HasSuffix(host, rule) {
			return true
		}
		if strings.HasPrefix(rule, "*.") && strings.HasSuffix(host, strings.TrimPrefix(rule, "*")) {
			return true
		}
	}
	return false
}

func HTTPClient(base *http.Client, cfg ProxyConfig) (*http.Client, error) {
	if base != nil {
		return base, nil
	}
	proxy, err := ProxyFunc(cfg)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxy
	return &http.Client{Transport: transport}, nil
}
