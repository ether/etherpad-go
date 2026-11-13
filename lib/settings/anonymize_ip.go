package settings

import (
	"net"
	"strings"
)

// IP logging modes (settings ipLogging, etherpad-lite).
const (
	IPLoggingFull      = "full"
	IPLoggingTruncated = "truncated"
	IPLoggingAnonymous = "anonymous"
)

// AnonymizeIP is how an IP address appears in logs: "full" keeps it,
// "truncated" zeroes the last IPv4 octet and keeps the first /48 of an IPv6
// address, "anonymous" (and any unknown mode) gives "ANONYMOUS". Something
// that is no IP address is always "ANONYMOUS".
func AnonymizeIP(ip, mode string) string {
	ip = strings.TrimSpace(ip)
	parsed := net.ParseIP(ip)
	if ip == "" || parsed == nil {
		return "ANONYMOUS"
	}
	switch mode {
	case IPLoggingFull:
		return ip
	case IPLoggingTruncated:
		if v4 := parsed.To4(); v4 != nil {
			truncated := net.IPv4(v4[0], v4[1], v4[2], 0).To4().String()
			if strings.Contains(ip, ":") { // v4-mapped IPv6
				return "::ffff:" + truncated
			}
			return truncated
		}
		masked := parsed.Mask(net.CIDRMask(48, 128))
		return masked.String()
	default:
		return "ANONYMOUS"
	}
}
