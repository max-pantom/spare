package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/state"
	"github.com/spare-run/spare/internal/supervisor"
)

func TestAPITokenAndSingleUseBrowserSession(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	machine := model.Machine{
		ID:             "spare_test",
		Hostname:       "test",
		InitializedAt:  time.Now().UTC(),
		LastProfiledAt: time.Now().UTC(),
	}
	if err := store.SaveMachine(context.Background(), machine); err != nil {
		t.Fatal(err)
	}
	manager, err := supervisor.New(store, t.TempDir(), "not-used", machine)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()

	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<h1>Spare</h1>"), Mode: fs.FileMode(0o600)},
	}
	server := httptest.NewServer(NewServer("secret-token", store, manager, assets).Handler())
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated health to return 401, got %d", response.StatusCode)
	}
	_ = response.Body.Close()

	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/browser-sessions", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if created.URL == "" {
		t.Fatal("browser session URL was empty")
	}

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	response, err = client.Get(created.URL)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected exchange redirect, got %d", response.StatusCode)
	}
	cookies := response.Cookies()
	_ = response.Body.Close()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected session cookie: %#v", cookies)
	}

	response, err = client.Get(created.URL)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected replayed code to return 401, got %d", response.StatusCode)
	}
	_ = response.Body.Close()

	machineRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/machine", nil)
	machineRequest.AddCookie(cookies[0])
	response, err = http.DefaultClient.Do(machineRequest)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected cookie-authenticated request to return 200, got %d", response.StatusCode)
	}
	_ = response.Body.Close()

	badOrigin, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/instances", strings.NewReader("{}"))
	badOrigin.AddCookie(cookies[0])
	badOrigin.Header.Set("Origin", "http://example.invalid")
	response, err = http.DefaultClient.Do(badOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected cross-origin mutation to return 403, got %d", response.StatusCode)
	}
	_ = response.Body.Close()

	parsed, _ := url.Parse(server.URL)
	if strings.Contains(created.URL, "secret-token") || !strings.Contains(created.URL, parsed.Host) {
		t.Fatalf("browser URL leaked the API token or used the wrong host: %s", created.URL)
	}
}
