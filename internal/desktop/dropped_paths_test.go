package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDescribeDroppedPathsClassifiesCanonicalItems(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "Website")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	recipePackage := filepath.Join(root, "drop.SP")
	backup := filepath.Join(root, "drop.spare-backup")
	regular := filepath.Join(root, "report.txt")
	for _, path := range []string{recipePackage, backup, regular} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "website-link")
	if err := os.Symlink(directory, link); err != nil {
		t.Fatal(err)
	}

	items, err := describeDroppedPaths([]string{link, recipePackage, backup, regular})
	if err != nil {
		t.Fatal(err)
	}
	expectedKinds := []string{"directory", "recipe-package", "backup", "file"}
	for index, expected := range expectedKinds {
		if items[index].Kind != expected {
			t.Fatalf("item %d kind = %q, want %q", index, items[index].Kind, expected)
		}
	}
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Path != resolvedDirectory {
		t.Fatalf("directory path = %q, want %q", items[0].Path, resolvedDirectory)
	}
}

func TestDescribeDroppedPathsBoundsSelection(t *testing.T) {
	paths := make([]string, 101)
	if _, err := describeDroppedPaths(paths); err == nil {
		t.Fatal("expected a bounded file selection")
	}
}
