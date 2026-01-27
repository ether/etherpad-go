package stats

type HealthStatus string

const (
	StatusPass HealthStatus = "pass"
	StatusWarn HealthStatus = "warn"
	StatusFail HealthStatus = "fail"
)

type Check struct {
	Status     HealthStatus `json:"status"`
	Component  string       `json:"component,omitempty"`
	Observed   any          `json:"observedValue,omitempty"`
	ObservedAt string       `json:"observedAt,omitempty"`
	Output     string       `json:"output,omitempty"`
}

type HealthResponse struct {
	Status    HealthStatus       `json:"status"`
	Version   string             `json:"version,omitempty"`
	ReleaseID string             `json:"releaseId,omitempty"`
	ServiceID string             `json:"serviceId,omitempty"`
	Checks    map[string][]Check `json:"checks,omitempty"`
}

type Checker interface {
	Name() string
	Check() Check
}
