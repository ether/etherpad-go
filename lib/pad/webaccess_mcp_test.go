package pad

import "testing"

// The MCP server checks its tokens itself, under both forms of its URL.
func TestMCPChecksItself(t *testing.T) {
	for _, p := range []string{"/mcp", "/mcp/"} {
		if NeedsUser(p) {
			t.Errorf("%s goes through the pads' access check", p)
		}
	}
	for _, p := range []string{"/mcp/x", "/mcpx", "/p/mcp"} {
		if !NeedsUser(p) {
			t.Errorf("%s skips the pads' access check", p)
		}
	}
}
