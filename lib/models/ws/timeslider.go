package ws

import (
	"encoding/json"
	"strconv"
	"strings"
)

type ChangesetReq struct {
	Event string `json:"event"`
	Data  struct {
		Component string `json:"component"`
		Type      string `json:"type"`
		PadId     string `json:"padId"`
		Token     string `json:"token"`
		Data      struct {
			Start       RevNum `json:"start"`
			Granularity int    `json:"granularity"`
			// RequestID is echoed back as sent; etherpad-lite clients
			// send strings, etherpad-go's numbers.
			RequestID any `json:"requestID"`
		} `json:"data"`
	} `json:"data"`
}

// RevNum is a revision number from a client: a number, or a string that
// starts with one ("2", "1test" -> 1, as etherpad-lite's parseInt). Anything
// else is invalid (-1).
type RevNum int

func (r *RevNum) UnmarshalJSON(b []byte) error {
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		if n != float64(int(n)) {
			*r = -1
			return nil
		}
		*r = RevNum(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		*r = -1
		return nil
	}
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	v, err := strconv.Atoi(s[:end])
	if err != nil {
		*r = -1
		return nil
	}
	*r = RevNum(v)
	return nil
}
