package recipe

import (
	"fmt"
	"sort"

	"github.com/spare-run/spare/internal/model"
)

type Implementation interface {
	Manifest() Manifest
	ResolveConfig(input map[string]any) (map[string]any, error)
	Serve(config map[string]any, port, healthPort int) error
}

// StatefulImplementation is implemented by jobs that need private,
// runtime-managed storage in addition to any user-selected folder.
type StatefulImplementation interface {
	Implementation
	ServeState(config map[string]any, port, healthPort int, dataPath string) error
}

type Registry struct {
	implementations map[string]Implementation
}

func NewRegistry(values ...Implementation) (*Registry, error) {
	registry := &Registry{implementations: make(map[string]Implementation, len(values))}
	for _, implementation := range values {
		manifest := implementation.Manifest()
		if err := Validate(manifest); err != nil {
			return nil, fmt.Errorf("register %s: %w", manifest.ID, err)
		}
		if _, exists := registry.implementations[manifest.ID]; exists {
			return nil, fmt.Errorf("recipe %q is registered more than once", manifest.ID)
		}
		registry.implementations[manifest.ID] = implementation
	}
	return registry, nil
}

func (r *Registry) Get(id string) (Implementation, bool) {
	value, ok := r.implementations[id]
	return value, ok
}

func (r *Registry) Manifests() []Manifest {
	result := make([]Manifest, 0, len(r.implementations))
	for _, implementation := range r.implementations {
		result = append(result, implementation.Manifest())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (r *Registry) Models(machine model.Machine) []model.Recipe {
	manifests := r.Manifests()
	result := make([]model.Recipe, 0, len(manifests))
	for _, manifest := range manifests {
		result = append(result, manifest.Model(machine))
	}
	return result
}
