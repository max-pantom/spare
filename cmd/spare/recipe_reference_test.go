package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/recipes"
)

func TestBuiltInRecipeReferencesUseBundledPackage(t *testing.T) {
	statePaths := paths.FromRoot(t.TempDir())
	packageDirectory := filepath.Join(statePaths.Root, "recipes")
	if err := os.MkdirAll(packageDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	registry, err := recipes.Builtins()
	if err != nil {
		t.Fatal(err)
	}
	implementation, ok := registry.Get("site")
	if !ok {
		t.Fatal("Site is not registered")
	}
	manifest := implementation.Manifest()
	packagePath := filepath.Join(
		packageDirectory,
		manifest.ID+"_"+manifest.Version+".sp",
	)
	if _, err := recipe.Pack(
		filepath.Join("..", "..", "recipes", "site"),
		packagePath,
	); err != nil {
		t.Fatal(err)
	}

	application := &app{paths: statePaths}
	resolved, err := application.recipePackageReference("SITE")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != packagePath {
		t.Fatalf("resolved package = %q, want %q", resolved, packagePath)
	}
	loaded, builtIn, err := application.recipeManifestReference("site")
	if err != nil {
		t.Fatal(err)
	}
	if !builtIn || loaded.ID != "site" {
		t.Fatalf("loaded recipe = %#v, builtIn = %v", loaded, builtIn)
	}
}

func TestRecipeValidateAcceptsBuiltInID(t *testing.T) {
	var output bytes.Buffer
	application := &app{
		paths: paths.FromRoot(t.TempDir()),
		out:   &output,
		err:   &output,
	}
	command := application.rootCommand()
	command.SetArgs([]string{"recipe", "validate", "drop"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Drop 0.1.0 is a valid built-in recipe.") {
		t.Fatalf("unexpected validation output: %q", output.String())
	}
}

func TestRecipePackageReferenceExplainsUnknownID(t *testing.T) {
	application := &app{paths: paths.FromRoot(t.TempDir())}
	_, err := application.recipePackageReference("missing")
	if err == nil || !strings.Contains(err.Error(), "spare recipe list") {
		t.Fatalf("error = %v", err)
	}
}
