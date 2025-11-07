package ws

type UserLeaveData struct {
	Type string `json:"type"`
	Data struct {
		Type     string `json:"type"`
		UserInfo struct {
			ColorId string `json:"colorId"`
			UserId  string `json:"userId"`
		} `json:"userInfo"`
	} `json:"data"`
}
