package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/spare-run/spare/internal/apischema"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/recipes"
	spareRuntime "github.com/spare-run/spare/internal/runtime"
	"github.com/spare-run/spare/internal/runtime/native"
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
	registry, err := recipes.Builtins()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := supervisor.New(
		store,
		t.TempDir(),
		machine,
		registry,
		map[string]spareRuntime.Runtime{"native": &native.Driver{Executable: "not-used"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()

	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<h1>Spare</h1>"), Mode: fs.FileMode(0o600)},
	}
	server := httptest.NewServer(NewServer("secret-token", store, manager, assets).Handler())
	defer server.Close()

	schemaRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/schema", nil)
	schemaRequest.Header.Set("Authorization", "Bearer secret-token")
	schemaResponse, err := http.DefaultClient.Do(schemaRequest)
	if err != nil {
		t.Fatal(err)
	}
	var schemaDocument map[string]any
	if err := json.NewDecoder(schemaResponse.Body).Decode(&schemaDocument); err != nil {
		t.Fatal(err)
	}
	_ = schemaResponse.Body.Close()
	if schemaResponse.StatusCode != http.StatusOK || schemaDocument["$id"] != apischema.ID {
		t.Fatalf("schema response = %d %#v", schemaResponse.StatusCode, schemaDocument)
	}

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

	trailingJSON, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/instances", strings.NewReader(`{} {}`))
	trailingJSON.Header.Set("Authorization", "Bearer secret-token")
	response, err = http.DefaultClient.Do(trailingJSON)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected multiple JSON values to return 400, got %d", response.StatusCode)
	}
	var envelope model.ErrorEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if envelope.Error.Code != "invalid_request" {
		t.Fatalf("error code = %q", envelope.Error.Code)
	}

	desktopOnly, _ := http.NewRequest(
		http.MethodPost,
		server.URL+"/api/v1/desktop/drop-files",
		strings.NewReader(`{"instanceId":"drop","paths":["/tmp/example"]}`),
	)
	desktopOnly.AddCookie(cookies[0])
	response, err = http.DefaultClient.Do(desktopOnly)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("browser session accessed desktop filesystem API: %d", response.StatusCode)
	}
	envelope = model.ErrorEnvelope{}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if envelope.Error.Code != "desktop_only" {
		t.Fatalf("desktop-only error code = %q", envelope.Error.Code)
	}

	localOnly, _ := http.NewRequest(
		http.MethodPost,
		server.URL+"/api/v1/instances",
		strings.NewReader(`{"recipeId":"site","mode":"installed","config":{"path":"/tmp"},"portMode":"auto","port":0}`),
	)
	localOnly.AddCookie(cookies[0])
	localOnly.Header.Set("Origin", server.URL)
	response, err = http.DefaultClient.Do(localOnly)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("browser session installed a recipe: %d", response.StatusCode)
	}
	envelope = model.ErrorEnvelope{}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if envelope.Error.Code != "local_user_required" {
		t.Fatalf("local-only error code = %q", envelope.Error.Code)
	}

	parsed, _ := url.Parse(server.URL)
	if strings.Contains(created.URL, "secret-token") || !strings.Contains(created.URL, parsed.Host) {
		t.Fatalf("browser URL leaked the API token or used the wrong host: %s", created.URL)
	}

	streamContext, cancelStream := context.WithCancel(context.Background())
	defer cancelStream()
	received := make(chan model.Event, 1)
	streamErrors := make(chan error, 1)
	go func() {
		streamErrors <- NewClient(server.URL, "secret-token").StreamActivity(
			streamContext,
			func(event model.Event) {
				select {
				case received <- event:
				default:
				}
			},
		)
	}()

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	eventTicker := time.NewTicker(25 * time.Millisecond)
	defer eventTicker.Stop()
	for {
		select {
		case <-eventTicker.C:
			if err := store.AddEvent(context.Background(), model.Event{
				Level:   "info",
				Kind:    "drop_file_received",
				Message: "report.pdf was received.",
			}); err != nil {
				t.Fatal(err)
			}
		case event := <-received:
			if event.Kind != "drop_file_received" || event.ID == 0 {
				t.Fatalf("unexpected streamed event: %#v", event)
			}
			cancelStream()
			select {
			case err := <-streamErrors:
				if err != nil && !errors.Is(err, context.Canceled) {
					t.Fatalf("stream returned %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("activity stream did not stop after cancellation")
			}
			return
		case <-deadline.C:
			cancelStream()
			t.Fatal("timed out waiting for streamed activity")
		}
	}
}
