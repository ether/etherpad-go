package admin

import "encoding/json"

type SettingsResultMessage struct {
	Results json.RawMessage `json:"results"`
}
