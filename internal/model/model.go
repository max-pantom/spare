package model

import "time"

const (
	RecipeSite      = "site"
	RecipeDrop      = "drop"
	RecipeHook      = "hook"
	RecipeClipboard = "clipboard"
	RecipeDownloads = "downloads"
	RecipeMonitor   = "monitor"

	ModeTemporary = "temporary"
	ModeInstalled = "installed"

	DesiredRunning = "running"
	DesiredStopped = "stopped"

	StatusStarting = "starting"
	StatusHealthy  = "healthy"
	StatusDegraded = "degraded"
	StatusStopped  = "stopped"
	StatusFailed   = "failed"
	StatusRemoving = "removing"

	InstallationBundled   = "bundled"
	InstallationInstalled = "installed"
)

type Machine struct {
	ID                    string       `json:"id"`
	Hostname              string       `json:"hostname"`
	OS                    string       `json:"os"`
	Architecture          string       `json:"architecture"`
	LogicalCores          int          `json:"logicalCores"`
	MemoryTotalBytes      uint64       `json:"memoryTotalBytes"`
	StorageAvailableBytes uint64       `json:"storageAvailableBytes"`
	LANAddresses          []string     `json:"lanAddresses"`
	Capabilities          Capabilities `json:"capabilities"`
	InitializedAt         time.Time    `json:"initializedAt"`
	LastProfiledAt        time.Time    `json:"lastProfiledAt"`
}

type Capabilities struct {
	CanServeLAN        bool `json:"canServeLAN"`
	CanRunPersistent   bool `json:"canRunPersistent"`
	CanStoreLargeFiles bool `json:"canStoreLargeFiles"`
	CanRunContainers   bool `json:"canRunContainers"`
	HasBattery         bool `json:"hasBattery"`
	HasExternalStorage bool `json:"hasExternalStorage"`
}

type ResourceGuidance struct {
	MemoryRecommendedBytes uint64 `json:"memoryRecommendedBytes"`
	MemoryMaximumBytes     uint64 `json:"memoryMaximumBytes"`
	CPUMaximum             int    `json:"cpuMaximum"`
}

type Compatibility struct {
	Supported bool     `json:"supported"`
	Rating    string   `json:"rating"`
	Reasons   []string `json:"reasons"`
	Warnings  []string `json:"warnings"`
}

type ConfigField struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

type PermissionGrant struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Granted     bool   `json:"granted"`
}

type Recipe struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Version          string            `json:"version"`
	Description      string            `json:"description"`
	Runtime          string            `json:"runtime"`
	SupportedSystems []string          `json:"supportedSystems"`
	Resources        ResourceGuidance  `json:"resources"`
	Config           []ConfigField     `json:"config"`
	Permissions      []PermissionGrant `json:"permissions"`
	Compatibility    Compatibility     `json:"compatibility"`
	Installation     string            `json:"installation"`
	Publisher        string            `json:"publisher,omitempty"`
	PackageVersion   string            `json:"packageVersion,omitempty"`
	MinimumSpare     string            `json:"minimumSpareVersion,omitempty"`
	Checksum         string            `json:"checksum,omitempty"`
	SignatureStatus  string            `json:"signatureStatus,omitempty"`
}

type JobPackage struct {
	ID              string    `json:"id"`
	Version         string    `json:"version"`
	Publisher       string    `json:"publisher"`
	MinimumSpare    string    `json:"minimumSpareVersion"`
	Checksum        string    `json:"checksum"`
	Signature       string    `json:"signature"`
	SignatureStatus string    `json:"signatureStatus"`
	ManifestJSON    []byte    `json:"-"`
	PackagePath     string    `json:"-"`
	Source          string    `json:"source,omitempty"`
	InstalledAt     time.Time `json:"installedAt"`
}

type JobPackageReview struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Version          string            `json:"version"`
	Description      string            `json:"description"`
	Publisher        string            `json:"publisher"`
	MinimumSpare     string            `json:"minimumSpareVersion"`
	Checksum         string            `json:"checksum"`
	SignatureStatus  string            `json:"signatureStatus"`
	Permissions      []PermissionGrant `json:"permissions"`
	AlreadyInstalled bool              `json:"alreadyInstalled"`
}

type JobProfile struct {
	RecipeID  string         `json:"recipeId"`
	Config    map[string]any `json:"config"`
	Port      int            `json:"port"`
	PortMode  string         `json:"portMode"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type Problem struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Recovery string `json:"recovery"`
}

type Instance struct {
	ID                    string         `json:"id"`
	RecipeID              string         `json:"recipeId"`
	Version               string         `json:"version"`
	Runtime               string         `json:"runtime"`
	Mode                  string         `json:"mode"`
	DesiredState          string         `json:"desiredState"`
	Status                string         `json:"status"`
	RootPath              string         `json:"rootPath"`
	DataPath              string         `json:"dataPath"`
	StatePath             string         `json:"-"`
	Config                map[string]any `json:"config"`
	Port                  int            `json:"port"`
	PortMode              string         `json:"portMode"`
	URLs                  []string       `json:"urls"`
	StorageAvailableBytes uint64         `json:"storageAvailableBytes"`
	ItemCount             int            `json:"itemCount"`
	StartedAt             *time.Time     `json:"startedAt,omitempty"`
	CreatedAt             time.Time      `json:"createdAt"`
	UpdatedAt             time.Time      `json:"updatedAt"`
	Problem               *Problem       `json:"problem,omitempty"`
}

type Event struct {
	ID         int64          `json:"id"`
	InstanceID string         `json:"instanceId,omitempty"`
	Level      string         `json:"level"`
	Kind       string         `json:"kind"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type ErrorEnvelope struct {
	Error APIError `json:"error"`
}
