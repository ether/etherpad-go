package error

type Error struct {
	Message string `json:"message"`
	Error   int    `json:"error"`
}
