package models

import "time"

type AppState string

const (
	AppStateStarted   AppState = "STARTED"
	AppStateStopped   AppState = "STOPPED"
	AppStateUploading AppState = "UPLOADING"
	AppStateStaging   AppState = "STAGING"
	AppStateDeploying AppState = "DEPLOYING"
	AppStateFailed    AppState = "FAILED"
)

type LifecycleType string

const (
	LifecycleBuildpack LifecycleType = "buildpack"
	LifecycleDocker    LifecycleType = "docker"
	LifecycleCNB       LifecycleType = "cnb"
)

// App is the central entity mapping CF Application + Process + Droplet into one struct.
type App struct {
	GUID          string            `json:"guid"`
	Name          string            `json:"name"`
	State         AppState          `json:"state"`
	LifecycleType LifecycleType     `json:"lifecycle_type"`
	Buildpacks    []string          `json:"buildpacks,omitempty"`
	DockerImage   string            `json:"docker_image,omitempty"`
	Command       string            `json:"command,omitempty"`
	Instances     int               `json:"instances"`
	MemoryMB      int               `json:"memory_mb"`
	DiskMB        int               `json:"disk_mb"`
	Port          int               `json:"port"`
	Env           map[string]string `json:"env,omitempty"`
	ImageRef      string            `json:"image_ref,omitempty"`
	HealthCheck   HealthCheck       `json:"health_check"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// AppDefaults returns an App with sensible defaults.
func AppDefaults(name string) App {
	return App{
		Name:          name,
		State:         AppStateStopped,
		LifecycleType: LifecycleBuildpack,
		Instances:     1,
		MemoryMB:      256,
		DiskMB:        1024,
		Port:          8080,
		Env:           make(map[string]string),
		HealthCheck:   HealthCheck{Type: HealthCheckPort},
	}
}

type AppStatus struct {
	App
	Routes       []Route          `json:"routes"`
	RunningCount int              `json:"running_count"`
	TotalCount   int              `json:"total_count"`
	Instances    []InstanceStatus `json:"instances,omitempty"`
}

type InstanceStatus struct {
	Index    int       `json:"index"`
	State    string    `json:"state"`
	Since    time.Time `json:"since"`
	CPU      float64   `json:"cpu"`
	MemoryMB int       `json:"memory_mb"`
	DiskMB   int       `json:"disk_mb"`
}
