package site

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHandlerServesIndexWithoutDirectoryListing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<h1>Hello</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(root)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Hello") {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/empty/", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected an empty directory to return 404, got %d", response.Code)
	}
}

func TestHandlerDeniesDotfilesAndTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=value"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{"/.env", "/%2eenv", "/../.env"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", target, response.Code)
		}
		if strings.Contains(response.Body.String(), "SECRET") {
			t.Errorf("%s exposed hidden file content", target)
		}
	}
}

func TestHandlerKeepsSymlinksInsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows requires optional privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "inside.txt"), filepath.Join(root, "safe.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "outside.txt"), filepath.Join(root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(root)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/safe.txt", nil))
	if response.Code != http.StatusOK || response.Body.String() != "inside" {
		t.Fatalf("expected safe symlink to be served, got %d %q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/escape.txt", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected escaping symlink to return 404, got %d", response.Code)
	}
}

func TestHandlerRejectsWrites(t *testing.T) {
	handler, err := NewHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
}

func TestSiteHealthDegradesAndRecoversWithRootFolder(t *testing.T) {
	root := t.TempDir()
	if snapshot := siteHealth(root); snapshot.Status != "healthy" {
		t.Fatalf("healthy snapshot = %#v", snapshot)
	}

	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	snapshot := siteHealth(root)
	if snapshot.Status != "degraded" ||
		snapshot.ProblemCode != "selected_folder_unavailable" {
		t.Fatalf("missing-folder snapshot = %#v", snapshot)
	}

	if err := os.Rename(moved, root); err != nil {
		t.Fatal(err)
	}
	if snapshot := siteHealth(root); snapshot.Status != "healthy" {
		t.Fatalf("recovered snapshot = %#v", snapshot)
	}
}
