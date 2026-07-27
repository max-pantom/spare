package hook

import (
	"github.com/spare-run/spare/internal/config"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/recipe"
)

type Implementation struct{}

func New() *Implementation {
	return &Implementation{}
}

func (i *Implementation) Manifest() recipe.Manifest {
	return recipe.Manifest{
		Schema:      recipe.SchemaV1,
		ID:          "hook",
		Name:        "Hook",
		Version:     "0.1.0",
		Description: "Receive, inspect, and replay webhook requests on the local network.",
		Support: recipe.SupportSpec{
			Systems:       []string{"darwin", "windows", "linux"},
			Architectures: []string{"amd64", "arm64"},
		},
		Runtime: recipe.RuntimeSpec{Type: "native"},
		Resources: recipe.ResourceSpec{
			MemoryRecommendedBytes: 64 * 1024 * 1024,
			MemoryMaximumBytes:     256 * 1024 * 1024,
			CPUMaximum:             1,
		},
		Network: recipe.NetworkSpec{Visibility: "local", Port: "automatic"},
		Health: recipe.HealthSpec{
			Type:             "http",
			Path:             "/",
			IntervalSeconds:  10,
			FailureThreshold: 3,
		},
		Config: map[string]config.Field{},
		Permissions: permissions.Set{
			Network:         permissions.Network{Local: true, Internet: true},
			StartOnLogin:    true,
			RunInBackground: true,
		},
	}
}

func (i *Implementation) ResolveConfig(input map[string]any) (map[string]any, error) {
	return config.Resolve(i.Manifest().Config, input)
}

func (i *Implementation) Serve(_ map[string]any, port, healthPort int) error {
	return newServer().serve(port, healthPort)
}
