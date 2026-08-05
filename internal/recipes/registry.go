package recipes

import (
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/recipes/clipboard"
	"github.com/spare-run/spare/internal/recipes/downloads"
	"github.com/spare-run/spare/internal/recipes/drop"
	"github.com/spare-run/spare/internal/recipes/hook"
	"github.com/spare-run/spare/internal/recipes/monitor"
	"github.com/spare-run/spare/internal/recipes/site"
)

func Builtins() (*recipe.Registry, error) {
	return recipe.NewRegistry(
		site.New(),
		drop.New(),
		hook.New(),
	)
}

// Trusted includes every implementation compiled into this Spare release.
// Optional implementations remain unavailable until their signed catalog
// package has been installed.
func Trusted() (*recipe.Registry, error) {
	return recipe.NewRegistry(
		site.New(),
		drop.New(),
		hook.New(),
		clipboard.New(),
		downloads.New(),
		monitor.New(),
	)
}
