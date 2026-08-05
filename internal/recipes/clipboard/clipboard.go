package clipboard

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
		ID:          "clipboard",
		Name:        "Clipboard",
		Version:     "0.1.0",
		Description: "Move text, links, and small files between trusted devices.",
		Support: recipe.SupportSpec{
			Systems:       []string{"darwin", "windows", "linux"},
			Architectures: []string{"amd64", "arm64"},
		},
		Runtime: recipe.RuntimeSpec{Type: "native"},
		Resources: recipe.ResourceSpec{
			MemoryRecommendedBytes: 64 * 1024 * 1024,
			MemoryMaximumBytes:     256 * 1024 * 1024,
			CPUMaximum:             1,
			StorageMinimumBytes:    25 * 1024 * 1024,
		},
		Network: recipe.NetworkSpec{Visibility: "local", Port: "automatic"},
		Health: recipe.HealthSpec{
			Type:             "http",
			Path:             "/",
			IntervalSeconds:  10,
			FailureThreshold: 3,
		},
		Config: map[string]config.Field{
			"pairing-code": {
				Type:        config.TypeString,
				Label:       "Pairing code",
				Description: "Leave blank to generate a six-digit code for trusted devices.",
			},
			"max-file-size": {
				Type:        config.TypeSize,
				Label:       "Maximum file size",
				Description: "Clipboard rejects individual files larger than this limit.",
				Default:     "25MB",
				Minimum:     1024,
				Maximum:     250_000_000,
			},
			"default-expiry": {
				Type:        config.TypeInteger,
				Label:       "Default expiry in minutes",
				Description: "New entries are removed after this amount of time.",
				Default:     60,
				Minimum:     5,
				Maximum:     1440,
			},
		},
		Permissions: permissions.Set{
			Network:         permissions.Network{Local: true, Internet: false},
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
	return errors.New("Clipboard requires runtime-managed private storage")
}

func (i *Implementation) ServeState(values map[string]any, port, healthPort int, dataPath string) error {
	server, err := newServer(values, dataPath)
	if err != nil {
		return err
	}
	return server.serve(port, healthPort)
}
