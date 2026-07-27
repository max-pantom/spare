package recipes

import (
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/recipes/drop"
	"github.com/spare-run/spare/internal/recipes/hook"
	"github.com/spare-run/spare/internal/recipes/site"
)

func Builtins() (*recipe.Registry, error) {
	return recipe.NewRegistry(
		site.New(),
		drop.New(),
		hook.New(),
	)
}
