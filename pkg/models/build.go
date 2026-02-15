package models

import "time"

type BuildState string

const (
	BuildStatePending BuildState = "PENDING"
	BuildStateStaging BuildState = "STAGING"
	BuildStateStaged  BuildState = "STAGED"
	BuildStateFailed  BuildState = "FAILED"
)

// Build represents a source-to-image build operation.
// Merges CF's Package + Build + Droplet into a single record.
type Build struct {
	GUID      string     `json:"guid"`
	AppGUID   string     `json:"app_guid"`
	State     BuildState `json:"state"`
	ImageRef  string     `json:"image_ref,omitempty"`
	Logs      string     `json:"logs,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
