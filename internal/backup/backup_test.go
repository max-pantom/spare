package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spare-run/spare/internal/model"
)

func TestExportInspectAndImport(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "hello.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "drop.spare-backup")
	err := ExportInstance(model.Instance{
		RecipeID: "drop",
		Version:  "0.1.0",
		Runtime:  "native",
		DataPath: source,
		Config: map[string]any{
			"destination":  source,
			"pairing-code": "123456",
		},
		PortMode: "auto",
	}, archive)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Inspect(archive)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.RecipeID != "drop" {
		t.Fatalf("recipe = %q", manifest.RecipeID)
	}
	if _, included := manifest.Config["pairing-code"]; included {
		t.Fatal("backup included the pairing code")
	}
	destination := filepath.Join(t.TempDir(), "restore")
	if _, err := Import(archive, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("data = %q", data)
	}
}

func TestExportRejectsRecipeWithoutSelectedFolder(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "hook.spare-backup")
	err := ExportInstance(model.Instance{
		RecipeID: "hook",
		Version:  "0.1.0",
		Runtime:  "native",
		Config:   map[string]any{},
		PortMode: "auto",
	}, archive)
	if err == nil {
		t.Fatal("expected an in-memory recipe export to be rejected")
	}
	if _, statErr := os.Stat(archive); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected backup file: %v", statErr)
	}
}
