package model

import "time"

const (
	RecipeSite = "site"

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
)

type Machine struct {
	ID                    string    `json:"id"`
	Hostname              string    `json:"hostname"`
	OS                    string    `json:"os"`
	Architecture          string    `json:"architecture"`
	LogicalCores          int       `json:"logicalCores"`
	MemoryTotalBytes      uint64    `json:"memoryTotalBytes"`
	StorageAvailableBytes uint64    `json:"storageAvailableBytes"`
	LANAddresses          []string  `json:"lanAddresses"`
	InitializedAt         time.Time `json:"initializedAt"`
	LastProfiledAt        time.Time `json:"lastProfiledAt"`
}

type ResourceGuidance struct {
	MemoryRecommendedBytes uint64 `json:"memoryRecommendedBytes"`
	MemoryMaximumBytes     uint64 `json:"memoryMaximumBytes"`
	CPUMaximum             int    `json:"cpuMaximum"`
}

type Recipe struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	Runtime          string           `json:"runtime"`
	SupportedSystems []string         `json:"supportedSystems"`
	Resources        ResourceGuidance `json:"resources"`
}

type Problem struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Recovery string `json:"recovery"`
}

type Instance struct {
	ID           string     `json:"id"`
	RecipeID     string     `json:"recipeId"`
	Mode         string     `json:"mode"`
	DesiredState string     `json:"desiredState"`
	Status       string     `json:"status"`
	RootPath     string     `json:"rootPath"`
	Port         int        `json:"port"`
	PortMode     string     `json:"portMode"`
	URLs         []string   `json:"urls"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	Problem      *Problem   `json:"problem,omitempty"`
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

func SiteRecipe() Recipe {
	return Recipe{
		ID:          RecipeSite,
		Title:       "Site",
		Description: "Serve a folder as a read-only website on this computer and the local network.",
		Runtime:     "native",
		SupportedSystems: []string{
			"darwin",
			"windows",
			"linux",
		},
		Resources: ResourceGuidance{
			MemoryRecommendedBytes: 64 * 1024 * 1024,
			MemoryMaximumBytes:     256 * 1024 * 1024,
			CPUMaximum:             1,
		},
	}
}
