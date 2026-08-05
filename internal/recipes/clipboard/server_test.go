package clipboard

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClipboardExpiresEntriesAndKeepsPrivateState(t *testing.T) {
	server, err := newServer(map[string]any{
		"pairing-code":   "123456",
		"max-file-size":  int64(25_000_000),
		"default-expiry": int64(60),
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.fileRoot.Close() })
	if err := server.addText("https://example.com", 60); err != nil {
		t.Fatal(err)
	}
	if server.entries[0].Kind != "link" {
		t.Fatalf("entry = %#v", server.entries[0])
	}
	server.mu.Lock()
	server.entries[0].ExpiresAt = time.Now().Add(-time.Minute)
	if err := server.saveLocked(); err != nil {
		t.Fatal(err)
	}
	server.mu.Unlock()
	server.cleanup()
	if len(server.entries) != 0 {
		t.Fatalf("entries = %#v", server.entries)
	}
	info, err := os.Stat(server.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %o", info.Mode().Perm())
	}
	if !filepath.IsAbs(server.root) {
		t.Fatalf("root = %s", server.root)
	}
}

func TestClipboardBoundsEntriesAndCleansMultipartFiles(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)
	server, err := newServer(map[string]any{
		"pairing-code":   "123456",
		"max-file-size":  int64(25_000_000),
		"default-expiry": int64(60),
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.fileRoot.Close() })

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", `..\unsafe/"name".txt`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(bytes.Repeat([]byte("a"), 2*1024*1024)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/entries", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.add(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(server.entries) != 1 ||
		strings.ContainsAny(server.entries[0].Name, `/\`) {
		t.Fatalf("stored entry = %#v", server.entries)
	}
	temporaryFiles, err := os.ReadDir(temporary)
	if err != nil {
		t.Fatal(err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("multipart temporary files remain: %#v", temporaryFiles)
	}

	server.mu.Lock()
	for len(server.entries) < maxClipboardEntries {
		server.entries = append(server.entries, entry{
			ID:        strings.Repeat("a", 23) + string(rune('a'+len(server.entries)%6)),
			Kind:      "text",
			Text:      "value",
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
	}
	server.mu.Unlock()
	if err := server.addText("one more", 60); err == nil {
		t.Fatal("expected the Clipboard entry limit to be enforced")
	}
}

func TestClipboardRejectsUnsafePersistedFilePath(t *testing.T) {
	root := t.TempDir()
	state := `[{"id":"0123456789abcdef01234567","kind":"file","name":"safe.txt","fileName":"../outside","size":1,"createdAt":"2026-01-01T00:00:00Z","expiresAt":"2027-01-01T00:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(root, "clipboard.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newServer(map[string]any{
		"pairing-code":   "123456",
		"max-file-size":  int64(25_000_000),
		"default-expiry": int64(60),
	}, root); err == nil {
		t.Fatal("expected an unsafe persisted file path to be rejected")
	}
}
