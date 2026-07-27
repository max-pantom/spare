package recipeview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spare-run/spare/internal/artifacts"
)

func TestViewerLoadsValidatedPackageAndListsFiles(t *testing.T) {
	viewer := testViewer(t)
	summary := viewer.Summary()
	if summary.Manifest.ID != "site" || summary.FileName != "site.sp" {
		t.Fatalf("summary = %#v", summary)
	}
	if len(summary.Files) != 3 {
		t.Fatalf("files = %#v", summary.Files)
	}
	if len(summary.SHA256) != 64 || summary.PackageSize == 0 || summary.UncompressedSize == 0 {
		t.Fatalf("package metadata = %#v", summary)
	}
	var icon File
	for _, file := range summary.Files {
		if file.Name == "icon.svg" {
			icon = file
		}
	}
	if icon.Preview != "text" || icon.MediaType != "text/plain; charset=utf-8" {
		t.Fatalf("SVG preview = %#v", icon)
	}
}

func TestViewerAPIRendersPackageFilesAsInertContent(t *testing.T) {
	viewer := testViewer(t)
	handler := viewer.Handler()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/package", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("package status = %d", response.Code)
	}
	var summary Summary
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Manifest.Name != "Site" || len(summary.Permissions) == 0 {
		t.Fatalf("package response = %#v", summary)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/file?name=README.md", nil))
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("README preview = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
	if !strings.Contains(response.Body.String(), "Site recipe") {
		t.Fatalf("README preview = %q", response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/file?name=icon.svg", nil))
	if response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("SVG executed as image content: %q", response.Header().Get("Content-Type"))
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/file?name=missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing file status = %d", response.Code)
	}
}

func TestViewerDoesNotPreviewExecutables(t *testing.T) {
	source := t.TempDir()
	manifest, err := os.ReadFile(filepath.Join("..", "..", "recipes", "site", "spare.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "spare.yml"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "worker.bin"), []byte{0, 1, 2, 3}, 0o700); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "site.sp")
	if err := artifacts.PackDirectory(source, packagePath); err != nil {
		t.Fatal(err)
	}
	viewer, err := New(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	viewer.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/file?name=worker.bin", nil))
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("binary preview status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestViewerPageAndLoopbackServer(t *testing.T) {
	viewer := testViewer(t)
	response := httptest.NewRecorder()
	viewer.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Package contents") {
		t.Fatalf("page status = %d", response.Code)
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "object-src 'none'") {
		t.Fatal("viewer page is missing restrictive security headers")
	}

	running, err := viewer.Start()
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, running.URL+"/api/package", nil)
	if err != nil {
		t.Fatal(err)
	}
	networkResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, networkResponse.Body)
	_ = networkResponse.Body.Close()
	if networkResponse.StatusCode != http.StatusOK {
		t.Fatalf("loopback viewer status = %d", networkResponse.StatusCode)
	}
	if err := running.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestViewerRequiresSPPackage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recipe.zip")
	if err := os.WriteFile(path, []byte("not a package"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path); err == nil {
		t.Fatal("expected a non-.sp file to be rejected")
	}
}

func testViewer(t *testing.T) *Viewer {
	t.Helper()
	packagePath := filepath.Join(t.TempDir(), "site.sp")
	if err := artifacts.PackDirectory(filepath.Join("..", "..", "recipes", "site"), packagePath); err != nil {
		t.Fatal(err)
	}
	viewer, err := New(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	return viewer
}
