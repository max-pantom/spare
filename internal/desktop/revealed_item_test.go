package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRevealedItemStaysInsideDrop(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "report.pdf")
	if err := os.WriteFile(inside, []byte("report"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveRevealedItem(root, "report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(inside)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != expected {
		t.Fatalf("resolved = %q, want %q", resolved, expected)
	}
	for _, invalid := range []string{"../report.pdf", filepath.Join("nested", "report.pdf"), ""} {
		if _, err := resolveRevealedItem(root, invalid); err == nil {
			t.Fatalf("accepted invalid received-file name %q", invalid)
		}
	}
}

func TestResolveRevealedItemRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outsideRoot := t.TempDir()
	outside := filepath.Join(outsideRoot, "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "receipt.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if _, err := resolveRevealedItem(root, "receipt.txt"); err == nil {
		t.Fatal("accepted a received-file symlink that escapes the Drop folder")
	}
}
