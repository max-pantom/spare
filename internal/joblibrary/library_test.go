package joblibrary

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spare-run/spare/internal/jobpackage"
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/recipes/clipboard"
	"github.com/spare-run/spare/internal/state"
)

func TestInstallTrustedPackageWithoutChangingRuntime(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := recipe.NewRegistry(clipboard.New())
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.Open(filepath.Join(t.TempDir(), "spare.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	verifier := jobpackage.NewVerifier(map[string]ed25519.PublicKey{
		jobpackage.DefaultKeyID: privateKey.Public().(ed25519.PublicKey),
	})
	library, err := New(store, t.TempDir(), "0.1.1-alpha.3", registry, verifier)
	if err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "clipboard.sp")
	if _, err := recipe.Pack(filepath.Join("..", "..", "recipes", "clipboard"), packagePath); err != nil {
		t.Fatal(err)
	}
	if _, err := jobpackage.Sign(packagePath, privateKey, "0.1.1-alpha.3"); err != nil {
		t.Fatal(err)
	}
	review, err := library.Review(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if review.ID != "clipboard" || review.SignatureStatus != "verified" || review.AlreadyInstalled {
		t.Fatalf("review = %#v", review)
	}
	installed, err := library.Install(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if !library.Available("clipboard") {
		t.Fatal("installed job is not available")
	}
	info, err := os.Stat(installed.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("package mode = %o", info.Mode().Perm())
	}
	review, err = library.Review(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if !review.AlreadyInstalled {
		t.Fatal("review did not report installed package")
	}
	if err := library.Uninstall(context.Background(), "clipboard"); err != nil {
		t.Fatal(err)
	}
	if library.Available("clipboard") {
		t.Fatal("uninstalled job is still available")
	}
	if _, err := os.Stat(installed.PackagePath); !os.IsNotExist(err) {
		t.Fatalf("installed package still exists: %v", err)
	}
}

func TestReviewRejectsPackageThatNeedsNewerSpare(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := recipe.NewRegistry(clipboard.New())
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.Open(filepath.Join(t.TempDir(), "spare.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	library, err := New(
		store,
		t.TempDir(),
		"0.1.1-alpha.3",
		registry,
		jobpackage.NewVerifier(map[string]ed25519.PublicKey{
			jobpackage.DefaultKeyID: privateKey.Public().(ed25519.PublicKey),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "clipboard.sp")
	if _, err := recipe.Pack(filepath.Join("..", "..", "recipes", "clipboard"), packagePath); err != nil {
		t.Fatal(err)
	}
	if _, err := jobpackage.Sign(packagePath, privateKey, "0.2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := library.Review(packagePath); err == nil {
		t.Fatal("expected incompatible package to be rejected")
	}
}

func TestReviewRejectsUnexpectedAndOversizedJobMetadata(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := recipe.NewRegistry(clipboard.New())
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.Open(filepath.Join(t.TempDir(), "spare.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	library, err := New(
		store,
		t.TempDir(),
		"0.1.1-alpha.3",
		registry,
		jobpackage.NewVerifier(map[string]ed25519.PublicKey{
			jobpackage.DefaultKeyID: privateKey.Public().(ed25519.PublicKey),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		change     func(*testing.T, string)
		errorMatch string
	}{
		{
			name: "unexpected file",
			change: func(t *testing.T, source string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(source, "payload.bin"), []byte("not metadata"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			errorMatch: "unexpected file",
		},
		{
			name: "oversized readme",
			change: func(t *testing.T, source string) {
				t.Helper()
				if err := os.WriteFile(
					filepath.Join(source, "README.md"),
					make([]byte, maxJobPackageFileBytes+1),
					0o600,
				); err != nil {
					t.Fatal(err)
				}
			},
			errorMatch: "exceeds",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			for _, name := range []string{"spare.yml", "README.md", "icon.svg"} {
				data, err := os.ReadFile(filepath.Join("..", "..", "recipes", "clipboard", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			test.change(t, source)
			packagePath := filepath.Join(t.TempDir(), "clipboard.sp")
			if _, err := recipe.Pack(source, packagePath); err != nil {
				t.Fatal(err)
			}
			if _, err := jobpackage.Sign(packagePath, privateKey, "0.1.1-alpha.3"); err != nil {
				t.Fatal(err)
			}
			if _, err := library.Review(packagePath); err == nil ||
				!strings.Contains(err.Error(), test.errorMatch) {
				t.Fatalf("review error = %v", err)
			}
		})
	}
}

func TestInstalledPackageIsReverifiedBeforeItBecomesAvailable(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := recipe.NewRegistry(clipboard.New())
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.Open(filepath.Join(t.TempDir(), "spare.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	library, err := New(
		store,
		t.TempDir(),
		"0.1.1-alpha.3",
		registry,
		jobpackage.NewVerifier(map[string]ed25519.PublicKey{
			jobpackage.DefaultKeyID: privateKey.Public().(ed25519.PublicKey),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "clipboard.sp")
	if _, err := recipe.Pack(filepath.Join("..", "..", "recipes", "clipboard"), packagePath); err != nil {
		t.Fatal(err)
	}
	if _, err := jobpackage.Sign(packagePath, privateKey, "0.1.1-alpha.3"); err != nil {
		t.Fatal(err)
	}
	installed, err := library.Install(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(installed.PackagePath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("tampered")); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if library.Available("clipboard") {
		t.Fatal("tampered installed package is available")
	}
	packages, err := library.Packages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].SignatureStatus != "invalid" {
		t.Fatalf("packages = %#v", packages)
	}
}
