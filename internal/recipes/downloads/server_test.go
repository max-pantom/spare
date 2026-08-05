package downloads

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDownloadResumesRangeAndCompletesInsideDestination(t *testing.T) {
	destination := t.TempDir()
	dataPath := t.TempDir()
	rangeSeen := ""
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		rangeSeen = request.Header.Get("Range")
		response.Header().Set("ETag", `"archive-v1"`)
		response.Header().Set("Content-Disposition", `attachment; filename="archive.zip"`)
		if rangeSeen == "bytes=3-" {
			response.Header().Set("Content-Length", "3")
			response.Header().Set("Content-Range", "bytes 3-5/6")
			response.WriteHeader(http.StatusPartialContent)
			_, _ = response.Write([]byte("def"))
			return
		}
		_, _ = response.Write([]byte("abcdef"))
	}))
	defer source.Close()

	server, err := newServer(map[string]any{
		"destination":  destination,
		"pairing-code": "123456",
	}, dataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer server.root.Close()
	server.client = source.Client()
	partial := filepath.Join(destination, ".spare-test.part")
	if err := os.WriteFile(partial, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	server.items = []item{{
		ID:          "test",
		URL:         source.URL + "/file",
		Name:        "file",
		PartialPath: partial,
		State:       stateDownloading,
		ETag:        `"archive-v1"`,
		CreatedAt:   time.Now().UTC(),
	}}
	server.download("test")
	if rangeSeen != "bytes=3-" {
		t.Fatalf("range = %q", rangeSeen)
	}
	completed := server.items[0]
	if completed.State != stateCompleted {
		t.Fatalf("item = %#v", completed)
	}
	data, err := os.ReadFile(completed.FinalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "abcdef" {
		t.Fatalf("data = %q", data)
	}
	if _, err := server.destinationName(completed.FinalPath); err != nil {
		t.Fatalf("completed path escaped destination: %s", completed.FinalPath)
	}
	info, err := os.Stat(completed.FinalPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("completed mode = %o", info.Mode().Perm())
	}
}

func TestDownloadURLAndFilenameValidation(t *testing.T) {
	for _, value := range []string{
		"file:///tmp/file",
		"https://name:password@example.com/file",
		"http://127.0.0.1/private",
		"http://192.168.1.1/private",
		"http://[::1]/private",
		"not a url",
	} {
		if _, err := validateURL(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "report.pdf"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := newServer(map[string]any{
		"destination":  root,
		"pairing-code": "123456",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer server.root.Close()
	if err := server.root.WriteFile(".spare-unique.part", []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := server.finalizeDownload(".spare-unique.part", "../report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "report (1).pdf" {
		t.Fatalf("path = %s", path)
	}
}

func TestCompletedDownloadOpensLocallyAndDownloadsRemotely(t *testing.T) {
	destination := t.TempDir()
	server, err := newServer(map[string]any{
		"destination":  destination,
		"pairing-code": "123456",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer server.root.Close()
	path := filepath.Join(destination, "report.pdf")
	if err := os.WriteFile(path, []byte("report"), 0o600); err != nil {
		t.Fatal(err)
	}
	server.items = []item{{
		ID:         "complete",
		Name:       "report.pdf",
		FinalPath:  path,
		State:      stateCompleted,
		Downloaded: 6,
		Total:      6,
		CreatedAt:  time.Now().UTC(),
	}}

	previousReveal := revealDownloadedFile
	t.Cleanup(func() { revealDownloadedFile = previousReveal })
	revealed := ""
	revealDownloadedFile = func(value string) error {
		revealed = value
		return nil
	}

	request := httptest.NewRequest(http.MethodPost, "/open/complete", nil)
	request.RemoteAddr = "127.0.0.1:54000"
	response := httptest.NewRecorder()
	server.openCompleted(response, request)
	if response.Code != http.StatusSeeOther || revealed != path {
		t.Fatalf("open status = %d, revealed = %q", response.Code, revealed)
	}

	request = httptest.NewRequest(http.MethodPost, "/open/complete", nil)
	request.RemoteAddr = "192.0.2.12:54000"
	response = httptest.NewRecorder()
	server.openCompleted(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("remote open status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:54000"
	response = httptest.NewRecorder()
	server.home(response, request)
	body := response.Body.String()
	for _, expected := range []string{
		`class="grid download-layout"`,
		`class="download-url-row"`,
		`action="/open/complete"`,
		`Show completed file`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("local page is missing %q", expected)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.12:54000"
	response = httptest.NewRecorder()
	server.home(response, request)
	if !strings.Contains(response.Body.String(), `href="/files/complete">Download file`) {
		t.Fatal("nearby page does not offer the completed file as a download")
	}
}

func TestDownloadHTTPClientRejectsLoopback(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("private"))
	}))
	defer source.Close()
	response, err := newDownloadHTTPClient().Get(source.URL)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil ||
		(!strings.Contains(err.Error(), "local") &&
			!strings.Contains(err.Error(), "private") &&
			!strings.Contains(err.Error(), "special-use")) {
		t.Fatalf("loopback request error = %v", err)
	}
}

func TestDownloadRejectsInvalidResumeRange(t *testing.T) {
	destination := t.TempDir()
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("ETag", `"changed"`)
		response.Header().Set("Content-Range", "bytes 0-2/3")
		response.WriteHeader(http.StatusPartialContent)
		_, _ = response.Write([]byte("bad"))
	}))
	defer source.Close()
	server, err := newServer(map[string]any{
		"destination":  destination,
		"pairing-code": "123456",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer server.root.Close()
	server.client = source.Client()
	partial := filepath.Join(destination, ".spare-range.part")
	if err := os.WriteFile(partial, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	server.items = []item{{
		ID:          "range",
		URL:         source.URL,
		Name:        "file",
		PartialPath: partial,
		State:       stateDownloading,
		ETag:        `"archive-v1"`,
		CreatedAt:   time.Now().UTC(),
	}}
	server.download("range")
	if server.items[0].State != stateFailed ||
		!strings.Contains(server.items[0].Error, "invalid resume range") {
		t.Fatalf("item = %#v", server.items[0])
	}
	data, err := os.ReadFile(partial)
	if err != nil || string(data) != "abc" {
		t.Fatalf("partial data = %q err=%v", data, err)
	}
}

func TestDownloadRejectsKnownOversizedResponse(t *testing.T) {
	destination := t.TempDir()
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", strconv.FormatInt(maxDownloadSize+1, 10))
		response.WriteHeader(http.StatusOK)
	}))
	defer source.Close()
	server, err := newServer(map[string]any{
		"destination":  destination,
		"pairing-code": "123456",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer server.root.Close()
	server.client = source.Client()
	server.items = []item{{
		ID:          "large",
		URL:         source.URL,
		Name:        "large",
		PartialPath: ".spare-large.part",
		State:       stateDownloading,
		CreatedAt:   time.Now().UTC(),
	}}
	server.download("large")
	if server.items[0].State != stateFailed ||
		!strings.Contains(server.items[0].Error, "file limit") {
		t.Fatalf("item = %#v", server.items[0])
	}
	if _, err := os.Stat(filepath.Join(destination, ".spare-large.part")); !os.IsNotExist(err) {
		t.Fatalf("oversized response created a partial file: %v", err)
	}
}
