package admin

type EventMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}
