package ws

type ChangesetReq struct {
	Event string `json:"event"`
	Data  struct {
		Component string `json:"component"`
		Type      string `json:"type"`
		PadId     string `json:"padId"`
		Token     string `json:"token"`
		Data      struct {
			Start       int    `json:"start"`
			Granularity int    `json:"granularity"`
			RequestID   string `json:"requestID"`
		} `json:"data"`
	} `json:"data"`
}
