package utils

import (
	"regexp"
	"strings"
)

// Headers naming the path prefix Etherpad is served under by a reverse
// proxy, in the order they are read. x-proxy-path (Etherpad's own) is
// always honoured, the operator has to configure the proxy to send it;
// the standard ones only with trustProxy, as any client could forge them
// when Etherpad is reachable directly.
var proxyPathHeaders = []struct {
	name         string
	requireTrust bool
}{
	{"x-proxy-path", false},
	{"x-forwarded-prefix", true}, // HAProxy, Traefik
	{"x-ingress-path", true},     // Home Assistant ingress
}

// ProxyPathHeaders are the headers ProxyPath reads, for a Vary header.
func ProxyPathHeaders(trustProxy bool) []string {
	var out []string
	for _, h := range proxyPathHeaders {
		if !h.requireTrust || trustProxy {
			out = append(out, h.name)
		}
	}
	return out
}

var (
	proxyPathDisallowed = regexp.MustCompile(`[^a-zA-Z0-9\-_/.]`)
	proxyPathTraversal  = regexp.MustCompile(`(?:^|/)\.\.(?:/|$)`)
)

// CleanProxyPath makes a header value safe to put in front of a path in
// HTML, JavaScript, CSS and Location headers: only [A-Za-z0-9-_./] stay,
// a leading // becomes / (no protocol-relative URL), a missing leading /
// is added, a value with a ".." segment is dropped and trailing slashes
// go. The result is "" or starts with exactly one "/" and does not end
// with one (etherpad-lite's sanitizeProxyPath, which keeps "/" and a
// trailing slash).
func CleanProxyPath(raw string) string {
	cleaned := proxyPathDisallowed.ReplaceAllString(raw, "")
	if cleaned == "" {
		return ""
	}
	if strings.HasPrefix(cleaned, "//") {
		cleaned = "/" + strings.TrimLeft(cleaned, "/")
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if proxyPathTraversal.MatchString(cleaned) {
		return ""
	}
	// Callers append "/..." to it: "/" or "/pads/" would make "//images"
	// (another host) or "/pads//images". "/" is no prefix at all.
	return strings.TrimRight(cleaned, "/")
}

// ProxyPath is the path prefix of the request: the first proxy path header
// that is honoured and not empty once cleaned, or "".
func ProxyPath(header func(name string) string, trustProxy bool) string {
	if header == nil {
		return ""
	}
	for _, h := range proxyPathHeaders {
		if h.requireTrust && !trustProxy {
			continue
		}
		if cleaned := CleanProxyPath(header(h.name)); cleaned != "" {
			return cleaned
		}
	}
	return ""
}
