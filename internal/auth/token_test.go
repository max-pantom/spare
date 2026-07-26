package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureTokenIsStableAndPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	first, err := EnsureToken(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EnsureToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) < 32 {
		t.Fatalf("token was not stable: %q %q", first, second)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
		}
	}
}
