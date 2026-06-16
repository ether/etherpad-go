package updater

import "strings"

// IsValidTag validates a release tag before it is interpolated into a URL or a
// filesystem path. It rejects empty/oversized tags, leading "-"/"." (option
// injection / hidden files), ".." (traversal), and control or shell-significant
// characters.
func IsValidTag(tag string) bool {
	if tag == "" || len(tag) > 200 {
		return false
	}
	if strings.HasPrefix(tag, "-") || strings.HasPrefix(tag, ".") {
		return false
	}
	if strings.Contains(tag, "..") {
		return false
	}
	for _, r := range tag {
		if r <= ' ' || r == 0x7f {
			return false
		}
		switch r {
		case '~', '^', ':', '?', '*', '[', '\\', '/':
			return false
		}
	}
	return true
}
