package policy

import (
	"net"
	"net/url"
	"strings"
)

func assessNetwork(req NetworkRequest) assessment {
	details := map[string]any{
		"url":          req.URL,
		"method":       req.Method,
		"resolved_ips": req.ResolvedIPs,
	}
	parsed, err := url.Parse(req.URL)
	if err != nil {
		details["error"] = err.Error()
		return assessment{Risk: RiskLevelCritical, Reason: "network URL could not be parsed", Details: details}
	}
	scheme := strings.ToLower(parsed.Scheme)
	details["scheme"] = scheme
	if scheme != "http" && scheme != "https" {
		return assessment{Risk: RiskLevelCritical, Reason: "network access is limited to HTTP(S)", Details: details}
	}

	host := parsed.Hostname()
	details["host"] = host
	if host == "" {
		return assessment{Risk: RiskLevelCritical, Reason: "network URL host is required", Details: details}
	}
	if isBlockedHostname(host) {
		return assessment{Risk: RiskLevelCritical, Reason: "network access to localhost, private IPs, and metadata services is blocked", Details: details}
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			details["ip"] = ip.String()
			return assessment{Risk: RiskLevelCritical, Reason: "network access to localhost, private IPs, and metadata services is blocked", Details: details}
		}
		details["ip"] = ip.String()
		return assessment{Risk: RiskLevelLow, Reason: "public HTTP(S) network access", Details: details}
	}

	for _, resolved := range req.ResolvedIPs {
		ip := net.ParseIP(resolved)
		if ip == nil {
			return assessment{Risk: RiskLevelCritical, Reason: "resolved network IP could not be parsed", Details: details}
		}
		if isBlockedIP(ip) {
			details["blocked_ip"] = ip.String()
			return assessment{Risk: RiskLevelCritical, Reason: "network access to localhost, private IPs, and metadata services is blocked", Details: details}
		}
	}

	return assessment{Risk: RiskLevelLow, Reason: "public HTTP(S) network access", Details: details}
}

func isBlockedHostname(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	switch host {
	case "localhost", "metadata.google.internal":
		return true
	}
	if strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return true
	}
	return false
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 0
	}
	return false
}
