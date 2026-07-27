package drop

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spare-run/spare/internal/config"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/recipe"
)

const defaultMaximumFileSize = int64(2_000_000_000)

type Implementation struct{}

func New() *Implementation {
	return &Implementation{}
}

func (i *Implementation) Manifest() recipe.Manifest {
	return recipe.Manifest{
		Schema:      recipe.SchemaV1,
		ID:          "drop",
		Name:        "Drop",
		Version:     "0.1.0",
		Description: "Send files to this computer from a browser on the local network.",
		Support: recipe.SupportSpec{
			Systems:       []string{"darwin", "windows", "linux"},
			Architectures: []string{"amd64", "arm64"},
		},
		Runtime: recipe.RuntimeSpec{Type: "native"},
		Resources: recipe.ResourceSpec{
			MemoryRecommendedBytes: 128 * 1024 * 1024,
			MemoryMaximumBytes:     512 * 1024 * 1024,
			CPUMaximum:             1,
			StorageMinimumBytes:    100 * 1024 * 1024,
		},
		Network: recipe.NetworkSpec{Visibility: "local", Port: "automatic"},
		Storage: recipe.StorageSpec{PathField: "destination", ReadOnly: false},
		Health: recipe.HealthSpec{
			Type:             "http",
			Path:             "/",
			IntervalSeconds:  10,
			FailureThreshold: 3,
		},
		Config: map[string]config.Field{
			"destination": {
				Type:        config.TypeDirectory,
				Label:       "Destination folder",
				Description: "Files received through Drop are written here.",
				Required:    true,
			},
			"max-file-size": {
				Type:        config.TypeSize,
				Label:       "Maximum file size",
				Description: "Drop rejects individual files larger than this limit.",
				Default:     "2GB",
				Minimum:     1024,
				Maximum:     100_000_000_000,
			},
		},
		Permissions: permissions.Set{
			Filesystem:      permissions.Filesystem{Read: []string{"destination"}, Write: []string{"destination"}},
			Network:         permissions.Network{Local: true, Internet: false},
			StartOnLogin:    true,
			RunInBackground: true,
		},
	}
}

func (i *Implementation) ResolveConfig(input map[string]any) (map[string]any, error) {
	resolved, err := config.Resolve(i.Manifest().Config, input)
	if err != nil {
		return nil, err
	}
	destination, err := validateDestination(resolved["destination"].(string))
	if err != nil {
		return nil, err
	}
	resolved["destination"] = destination
	return resolved, nil
}

func (i *Implementation) Serve(values map[string]any, port, healthPort int) error {
	destination, ok := values["destination"].(string)
	if !ok || destination == "" {
		return errors.New("the Drop worker is missing its destination folder")
	}
	maximum, err := numberValue(values["max-file-size"])
	if err != nil {
		return err
	}
	if maximum == 0 {
		maximum = defaultMaximumFileSize
	}
	server, err := newServer(destination, maximum)
	if err != nil {
		return err
	}
	return server.serve(port, healthPort)
}

func validateDestination(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("the Drop destination must be a folder")
	}
	probe, err := os.CreateTemp(resolved, ".spare-write-test-*")
	if err != nil {
		return "", errors.New("the Drop destination is not writable")
	}
	probeName := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probeName)
		return "", err
	}
	if err := os.Remove(probeName); err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func numberValue(value any) (int64, error) {
	if value == nil {
		return 0, nil
	}
	return config.ParseSize(value)
}
