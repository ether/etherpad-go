package legacy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every function of the latest version has a /rest path, and every path
// and /api/2 route names a function of its version.
func TestRESTTablesMatchTheFunctions(t *testing.T) {
	for _, f := range sortedFunctions(LatestVersion) {
		assert.NotEmpty(t, restPaths[f], "no /rest path for %s", f)
	}
	for f := range restPaths {
		_, ok := versions[LatestVersion][f]
		assert.True(t, ok, "/rest path for unknown function %s", f)
	}
	for _, r := range api2Routes {
		_, ok := versions[r.version][r.function]
		assert.True(t, ok, "/api/2 %s %s: %s is not in version %s", r.method, r.path, r.function, r.version)
	}
}
