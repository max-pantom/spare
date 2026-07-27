package desktop

import (
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/preferences"
)

type Snapshot struct {
	Surface     string           `json:"surface"`
	Machine     model.Machine    `json:"machine"`
	Recipes     []model.Recipe   `json:"recipes"`
	Instances   []model.Instance `json:"instances"`
	Events      []model.Event    `json:"events"`
	Preferences Preferences      `json:"preferences"`
}

type CreateInput struct {
	RecipeID string         `json:"recipeId"`
	Mode     string         `json:"mode"`
	Config   map[string]any `json:"config"`
	Port     int            `json:"port"`
	PortMode string         `json:"portMode"`
}

type Preferences = preferences.Desktop
