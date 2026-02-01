package stats

// HealthStatus represents the status of a health check
// @Description Status of a health check (pass, warn, fail)
type HealthStatus string

const (
	StatusPass HealthStatus = "pass"
	StatusWarn HealthStatus = "warn"
	StatusFail HealthStatus = "fail"
)

// Check represents the result of a single health check
// @Description Result of a health check
type Check struct {
	Status     HealthStatus `json:"status" example:"pass"`
	Component  string       `json:"component,omitempty" example:"database"`
	Observed   any          `json:"observedValue,omitempty"`
	ObservedAt string       `json:"observedAt,omitempty" example:"2024-01-15T10:30:00Z"`
	Output     string       `json:"output,omitempty" example:"ok"`
}

// HealthResponse represents the response of the health endpoint
// @Description Complete health check response
type HealthResponse struct {
	Status    HealthStatus       `json:"status" example:"pass"`
	Version   string             `json:"version,omitempty" example:"1.0.0"`
	ReleaseID string             `json:"releaseId,omitempty" example:"abc123"`
	ServiceID string             `json:"serviceId,omitempty" example:"etherpad-api"`
	Checks    map[string][]Check `json:"checks,omitempty"`
}

// Checker is the interface for health check implementations
type Checker interface {
	Name() string
	Check() Check
}
