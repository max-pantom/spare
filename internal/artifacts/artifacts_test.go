package artifacts

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackReadAndExtract(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "spare.yml"), []byte("schema: spare.recipe/v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "test.sp")
	if err := PackDirectory(source, packagePath); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFile(packagePath, "spare.yml")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "schema: spare.recipe/v1\n" {
		t.Fatalf("data = %q", data)
	}
	destination := t.TempDir()
	if err := Extract(packagePath, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "spare.yml")); err != nil {
		t.Fatal(err)
	}
}

func TestPackageContentIsReproducible(t *testing.T) {
	source := t.TempDir()
	manifest := filepath.Join(source, "spare.yml")
	if err := os.WriteFile(manifest, []byte("schema: spare.recipe/v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(t.TempDir(), "first.sp")
	second := filepath.Join(t.TempDir(), "second.sp")
	if err := PackDirectory(source, first); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(manifest, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := PackDirectory(source, second); err != nil {
		t.Fatal(err)
	}
	firstData, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstData, secondData) {
		t.Fatal("identical package content produced different archives")
	}
}

func TestDownloadAtomicallyReplacesExistingArtifact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", "3")
		_, _ = response.Write([]byte("new"))
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Download(context.Background(), server.Client(), server.URL, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("downloaded data = %q", data)
	}
}

func TestCacheCleansOnlyOldRegularFiles(t *testing.T) {
	root := t.TempDir()
	cache := Cache{Root: root}
	old := cache.Path("https://example.test/old")
	recent := cache.Path("https://example.test/recent")
	if err := os.WriteFile(old, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(old, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recent, []byte("recent"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cache.CleanOlderThan(time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old artifact still exists: %v", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatalf("recent artifact was removed: %v", err)
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	packagePath := filepath.Join(t.TempDir(), "bad.sp")
	file, err := os.Create(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	writer, err := archive.Create("../outside")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("no")); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Extract(packagePath, t.TempDir()); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestChecksumAndPlatformSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("spare"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, err := SHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, sum); err != nil {
		t.Fatal(err)
	}
	selected, err := Select(map[string]string{"linux-arm64": "spared"}, Platform{OS: "linux", Architecture: "arm64"})
	if err != nil || selected != "spared" {
		t.Fatalf("selected=%q err=%v", selected, err)
	}
}
