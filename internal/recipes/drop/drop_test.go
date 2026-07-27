package drop

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveConfigRequiresWritableDirectory(t *testing.T) {
	values, err := New().ResolveConfig(map[string]any{
		"destination":   t.TempDir(),
		"max-file-size": "5MB",
	})
	if err != nil {
		t.Fatal(err)
	}
	if values["max-file-size"] != int64(5_000_000) {
		t.Fatalf("size = %#v", values["max-file-size"])
	}
}

func TestUploadDownloadAndCollisionHandling(t *testing.T) {
	root := t.TempDir()
	server, err := newServer(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.routes()
	upload := func(name, content string) fileEntry {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("upload status = %d body=%s", response.Code, response.Body.String())
		}
		var entry fileEntry
		if err := json.Unmarshal(response.Body.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		return entry
	}
	first := upload("report.txt", "first")
	second := upload("report.txt", "second")
	if first.Name != "report.txt" || second.Name != "report (1).txt" {
		t.Fatalf("names = %q, %q", first.Name, second.Name)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, second.URL, nil))
	if response.Code != http.StatusOK || response.Body.String() != "second" {
		t.Fatalf("download = %d %q", response.Code, response.Body.String())
	}
}

func TestUploadRejectsHiddenName(t *testing.T) {
	root := t.TempDir()
	server, err := newServer(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", ".secret")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("secret"))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("hidden file status = %d", response.Code)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "secret") {
			t.Fatalf("unexpected file %q", entry.Name())
		}
	}
}

func TestUploadRejectsOversizedFile(t *testing.T) {
	root := t.TempDir()
	server, err := newServer(root, 4)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "report.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("longer"))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized file status = %d", response.Code)
	}
	if _, err := os.Stat(filepath.Join(root, "report.txt")); !os.IsNotExist(err) {
		t.Fatalf("oversized file was created: %v", err)
	}
}

func TestUploadAllowsOnlyOneTransferAtATime(t *testing.T) {
	server, err := newServer(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	server.uploadSlot <- struct{}{}
	defer func() { <-server.uploadSlot }()

	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/upload", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("concurrent upload status = %d", response.Code)
	}
}

func TestSafeFilenameRejectsPathsAndReservedCharacters(t *testing.T) {
	for _, name := range []string{
		"../secret.txt",
		`folder\secret.txt`,
		"unsafe:name.txt",
		" trailing.txt",
		"line\nbreak.txt",
	} {
		if safeFilename(name) != "" {
			t.Errorf("safeFilename(%q) was accepted", name)
		}
	}
	if safeFilename("résumé 2026.pdf") != "résumé 2026.pdf" {
		t.Fatal("safe Unicode filename was rejected")
	}
}

func TestDownloadRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := downloadPath(root, "escape.txt"); err == nil {
		t.Fatal("expected symlink rejection")
	}
}
