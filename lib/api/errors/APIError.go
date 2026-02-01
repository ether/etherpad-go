package errors

// Error represents an API error
// @Description Standardized API error response
type Error struct {
	Message string `json:"message" example:"Pad not found"`
	Error   int    `json:"error" example:"404"`
}
