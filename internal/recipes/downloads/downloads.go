package downloads

import (
	"errors"
	"os"
	"path/filepath"

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
		ID:          "downloads",
		Name:        "Downloads",
		Version:     "0.1.0",
		Description: "Download large files in the background.",
		Support: recipe.SupportSpec{
			Systems:       []string{"darwin", "windows", "linux"},
			Architectures: []string{"amd64", "arm64"},
		},
		Runtime: recipe.RuntimeSpec{Type: "native"},
		Resources: recipe.ResourceSpec{
			MemoryRecommendedBytes: 128 * 1024 * 1024,
			MemoryMaximumBytes:     512 * 1024 * 1024,
			CPUMaximum:             1,
			StorageMinimumBytes:    250 * 1024 * 1024,
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
				Label:       "Download folder",
				Description: "Completed files and resumable partial downloads are kept here.",
				Required:    true,
			},
			"pairing-code": {
				Type:        config.TypeString,
				Label:       "Pairing code",
				Description: "Leave blank to generate a six-digit code for trusted devices.",
			},
		},
		Permissions: permissions.Set{
			Filesystem: permissions.Filesystem{
				Read:  []string{"destination"},
				Write: []string{"destination"},
			},
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
	resolved, err := config.Resolve(i.Manifest().Config, withCode)
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
	return errors.New("Downloads requires runtime-managed private storage")
}

func (i *Implementation) ServeState(values map[string]any, port, healthPort int, dataPath string) error {
	server, err := newServer(values, dataPath)
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
		return "", errors.New("the download destination must be a folder")
	}
	probe, err := os.CreateTemp(resolved, ".spare-download-test-*")
	if err != nil {
		return "", errors.New("the download destination is not writable")
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
