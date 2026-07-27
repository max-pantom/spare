package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spare-run/spare/internal/model"
)

func TestStorageChecksDoNotRemoveSelectedFolder(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	checks := storageChecks(model.Instance{ID: "drop", RecipeID: "drop", DataPath: root})
	if len(checks) < 2 {
		t.Fatalf("checks = %#v", checks)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("doctor changed the selected folder")
	}
}
