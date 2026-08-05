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

func TestExportAndImportUnicodePaths(t *testing.T) {
	source := filepath.Join(t.TempDir(), "Source folder 東京")
	if err := os.MkdirAll(filepath.Join(source, "résumés"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "résumés", "你好 world.txt"), []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "Spare backups 東京", "Drop résumé.spare-backup")
	if err := Export(source, Manifest{Schema: SchemaV1, RecipeID: "drop"}, archive); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "Restored folder 東京")
	if _, err := Import(archive, destination); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "résumés", "你好 world.txt"))
	if err != nil || string(contents) != "safe" {
		t.Fatalf("restored Unicode file = %q, %v", contents, err)
	}
}

func TestFailedExportLeavesExistingBackupUntouched(t *testing.T) {
	source := t.TempDir()
	if err := os.Symlink("missing", filepath.Join(source, "interrupt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	destinationDir := t.TempDir()
	destination := filepath.Join(destinationDir, "drop.spare-backup")
	if err := os.WriteFile(destination, []byte("previous-good-backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Export(source, Manifest{Schema: SchemaV1, RecipeID: "drop"}, destination); err == nil {
		t.Fatal("expected export to fail after opening its temporary archive")
	}
	contents, err := os.ReadFile(destination)
	if err != nil || string(contents) != "previous-good-backup" {
		t.Fatalf("existing backup changed after failed export: %q, %v", contents, err)
	}
	entries, err := os.ReadDir(destinationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "drop.spare-backup" {
		t.Fatalf("partial backup was left behind: %#v", entries)
	}
}
