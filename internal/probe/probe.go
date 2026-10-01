// Package probe runs live checks against GitHub and local runner backends.
package probe

// Status is the outcome of one check.
type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusSkipped Status = "skipped"
)

// Check is one named step of a probe with its outcome.
type Check struct {
	Name    string
	Status  Status
	Message string
}
