package ws

type ClientReady struct {
	Event string `json:"event"`
	Data  struct {
		Component string `json:"component"`
		Type      string `json:"type"`
		PadID     string `json:"padId"`
		Token     string `json:"token"`
		UserInfo  struct {
			ColorId *string `json:"colorId"`
			Name    *string `json:"name"`
		} `json:"userInfo"`
	} `json:"data"`
}
