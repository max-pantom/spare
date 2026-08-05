package monitor

import (
	"errors"

	"github.com/spare-run/spare/internal/config"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/recipes/shared/pairing"
)

type Implementation struct{}

func New() *Implementation {
	return &Implementation{}
}

func (i *Implementation) Manifest() recipe.Manifest {
	return recipe.Manifest{
		Schema:      recipe.SchemaV1,
		ID:          "monitor",
		Name:        "Monitor",
		Version:     "0.1.0",
		Description: "Know when websites, devices, or local services go offline.",
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
			FailureThreshold: 2,
		},
		Config: map[string]config.Field{
			"pairing-code": {
				Type:        config.TypeString,
				Label:       "Pairing code",
				Description: "Leave blank to generate a six-digit code for trusted devices.",
			},
			"check-interval": {
				Type:        config.TypeInteger,
				Label:       "Check interval in seconds",
				Description: "Monitor checks every target at this interval.",
				Default:     30,
				Minimum:     10,
				Maximum:     3600,
			},
		},
		Permissions: permissions.Set{
			Network:         permissions.Network{Local: true, Internet: true},
			StartOnLogin:    true,
			RunInBackground: true,
		},
	}
}

func (i *Implementation) ResolveConfig(input map[string]any) (map[string]any, error) {
	withCode, err := pairing.WithGeneratedCode(input)
	if err != nil {
		return nil, err
	}
	return config.Resolve(i.Manifest().Config, withCode)
}

func (i *Implementation) Serve(values map[string]any, port, healthPort int) error {
	return errors.New("Monitor requires runtime-managed private storage")
}

func (i *Implementation) ServeState(values map[string]any, port, healthPort int, dataPath string) error {
	server, err := newServer(values, dataPath)
	if err != nil {
		return err
	}
	return server.serve(port, healthPort)
}
