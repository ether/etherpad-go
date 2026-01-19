package ws

type ClientReady struct {
	Event string          `json:"event"`
	Data  ClientReadyData `json:"data"`
}

type ClientReadyData struct {
	Component string              `json:"component"`
	Type      string              `json:"type"`
	PadID     string              `json:"padId"`
	Token     string              `json:"token"`
	UserInfo  ClientReadyUserInfo `json:"userInfo"`
	Reconnect *bool               `json:"reconnect"`
	ClientRev *int                `json:"client_rev"`
}

type ClientReadyUserInfo struct {
	ColorId *string `json:"colorId"`
	Name    *string `json:"name"`
}
