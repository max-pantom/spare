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
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := EnsureToken(path); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
		}
	}
}

func TestReadTokenRejectsMalformedAndNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	malformed := filepath.Join(root, "malformed")
	if err := os.WriteFile(malformed, []byte("this-is-not-a-256-bit-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadToken(malformed); err == nil {
		t.Fatal("expected a malformed token to be rejected")
	}
	if _, err := ReadToken(root); err == nil {
		t.Fatal("expected a directory token path to be rejected")
	}
	if runtime.GOOS != "windows" {
		valid := filepath.Join(root, "valid")
		if _, err := EnsureToken(valid); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "link")
		if err := os.Symlink(valid, link); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadToken(link); err == nil {
			t.Fatal("expected a symlinked token to be rejected")
		}
	}
}
