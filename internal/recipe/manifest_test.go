package recipe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spare-run/spare/internal/model"
)

const validManifest = `schema: spare.recipe/v1
id: example
name: Example
version: 1.0.0
description: Example recipe.
support:
  systems: [darwin, windows, linux]
  architectures: [amd64, arm64]
runtime:
  type: native
resources:
  memoryRecommendedBytes: 67108864
network:
  visibility: local
  port: automatic
storage:
  pathField: destination
config:
  destination:
    type: directory
    label: Destination
    required: true
permissions:
  filesystem:
    read: [destination]
  network:
    local: true
`

func TestParseAndCompatibility(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	result := manifest.Compatible(model.Machine{
		OS:                    "linux",
		Architecture:          "arm64",
		MemoryTotalBytes:      8 << 30,
		StorageAvailableBytes: 10 << 30,
		Capabilities:          model.Capabilities{CanServeLAN: true},
	})
	if !result.Supported || result.Rating != "Excellent" {
		t.Fatalf("compatibility = %#v", result)
	}
}

func TestLoadAndPack(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "spare.yml"), []byte(validManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "example.sp")
	manifest, err := Pack(source, output)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "example" {
		t.Fatalf("id = %q", manifest.ID)
	}
	loaded, err := Load(output)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != manifest.ID {
		t.Fatalf("loaded id = %q", loaded.ID)
	}
}

func TestValidateRejectsUnknownRuntime(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Runtime.Type = "container"
	if err := Validate(manifest); err == nil {
		t.Fatal("expected runtime validation error")
	}
}
