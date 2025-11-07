package ws

type UserNewInfoDat struct {
	UserId  string  `json:"userId"`
	Name    *string `json:"name"`
	ColorId string  `json:"colorId"`
}

type UserNewInfoData struct {
	Type     string         `json:"type"`
	UserInfo UserNewInfoDat `json:"userInfo"`
}

type UserNewInfo struct {
	Type string          `json:"type"`
	Data UserNewInfoData `json:"data"`
}
