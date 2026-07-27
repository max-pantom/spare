package instance

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/recipe"
)

type CreateRequest struct {
	RecipeID string
	Mode     string
	Config   map[string]any
	Port     int
	PortMode string
}

func Build(registry *recipe.Registry, request CreateRequest) (model.Instance, error) {
	implementation, ok := registry.Get(request.RecipeID)
	if !ok {
		return model.Instance{}, fmt.Errorf("unknown recipe %q", request.RecipeID)
	}
	if request.Mode != model.ModeTemporary && request.Mode != model.ModeInstalled {
		return model.Instance{}, errors.New("instance mode must be temporary or installed")
	}
	if request.PortMode == "" {
		request.PortMode = "auto"
	}
	if request.PortMode != "auto" && request.PortMode != "fixed" {
		return model.Instance{}, errors.New("port mode must be auto or fixed")
	}
	resolved, err := implementation.ResolveConfig(request.Config)
	if err != nil {
		return model.Instance{}, err
	}
	manifest := implementation.Manifest()
	dataPath := ""
	if manifest.Storage.PathField != "" {
		dataPath, _ = resolved[manifest.Storage.PathField].(string)
	}
	now := time.Now().UTC()
	return model.Instance{
		ID:           request.RecipeID,
		RecipeID:     request.RecipeID,
		Version:      manifest.Version,
		Runtime:      manifest.Runtime.Type,
		Mode:         request.Mode,
		DesiredState: model.DesiredRunning,
		Status:       model.StatusStarting,
		RootPath:     dataPath,
		DataPath:     dataPath,
		Config:       resolved,
		Port:         request.Port,
		PortMode:     request.PortMode,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func SameConfiguration(current model.Instance, request CreateRequest, candidate model.Instance) bool {
	return current.Mode == model.ModeInstalled &&
		request.Mode == model.ModeInstalled &&
		current.RecipeID == candidate.RecipeID &&
		current.Version == candidate.Version &&
		current.PortMode == candidate.PortMode &&
		(candidate.PortMode == "auto" || current.Port == request.Port) &&
		reflect.DeepEqual(current.Config, candidate.Config)
}
