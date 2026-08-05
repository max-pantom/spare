//go:build windows

package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecureAndVerifyPrivateTreeWindows(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "jobs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api-token"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SecurePrivateTree(root); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPrivateTree(root); err != nil {
		t.Fatal(err)
	}
}
