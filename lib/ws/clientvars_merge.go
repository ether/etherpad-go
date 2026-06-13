package ws

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib/models/clientVars"
)

// MergeClientVarsExtra serializes cv to a top-level JSON object and overlays the
// keys from extra that the typed struct does not already own. On collision the
// typed field wins, so plugins cannot clobber engine-owned CLIENT_VARS keys.
func MergeClientVarsExtra(cv *clientVars.ClientVars, extra map[string]any) (map[string]any, error) {
	base, err := json.Marshal(cv)
	if err != nil {
		return nil, err
	}
	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}
	for k, v := range extra {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}
	return merged, nil
}
