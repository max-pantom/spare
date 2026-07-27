package backup

import (
	"time"

	"github.com/spare-run/spare/internal/model"
)

const SchemaV1 = "spare.backup/v1"

type Manifest struct {
	Schema     string         `json:"schema"`
	RecipeID   string         `json:"recipeId"`
	Version    string         `json:"version"`
	Runtime    string         `json:"runtime"`
	Config     map[string]any `json:"config"`
	Port       int            `json:"port"`
	PortMode   string         `json:"portMode"`
	ExportedAt time.Time      `json:"exportedAt"`
}

func fromInstance(instance model.Instance) Manifest {
	return Manifest{
		Schema:     SchemaV1,
		RecipeID:   instance.RecipeID,
		Version:    instance.Version,
		Runtime:    instance.Runtime,
		Config:     instance.Config,
		Port:       instance.Port,
		PortMode:   instance.PortMode,
		ExportedAt: time.Now().UTC(),
	}
}
