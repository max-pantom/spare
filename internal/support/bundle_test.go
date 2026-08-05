package support

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/paths"
)

func TestBundleExcludesSecretsPathsAndNetworkIdentity(t *testing.T) {
	const secret = "SECRET_API_TOKEN_VALUE"
	const selected = "/Users/max/Private/received-files"
	const hostname = "max-secret-macbook"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/health":
			_, _ = io.WriteString(response, `{"status":"healthy"}`)
		case "/api/v1/machine":
			_, _ = io.WriteString(response, `{"id":"private-machine-id","hostname":"`+hostname+`","os":"darwin","architecture":"arm64","logicalCores":8,"memoryTotalBytes":16,"storageAvailableBytes":32,"lanAddresses":["192.168.1.20"],"capabilities":{},"initializedAt":"2026-01-01T00:00:00Z","lastProfiledAt":"2026-01-01T00:00:00Z"}`)
		case "/api/v1/instances":
			_, _ = io.WriteString(response, `[{"id":"drop","recipeId":"drop","mode":"installed","desiredState":"running","status":"healthy","dataPath":"`+selected+`","config":{"destination":"`+selected+`"},"urls":["http://192.168.1.20:7340"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]`)
		case "/api/v1/job-packages":
			_, _ = io.WriteString(response, `[]`)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	statePaths := paths.FromRoot(t.TempDir())
	if err := statePaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePaths.Token, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "support.zip")
	if _, err := Create(context.Background(), destination, "test", api.NewClient(server.URL, secret), statePaths); err != nil {
		t.Fatal(err)
	}

	reader, err := zip.OpenReader(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var combined strings.Builder
	for _, file := range reader.File {
		entry, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(entry)
		_ = entry.Close()
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(data)
		if file.Name == "support.json" {
			var value Bundle
			if err := json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			if len(value.Jobs) != 1 || value.Jobs[0].RecipeID != "drop" {
				t.Fatalf("jobs = %#v", value.Jobs)
			}
		}
	}
	text := combined.String()
	for _, forbidden := range []string{secret, selected, hostname, "private-machine-id", "192.168.1.20"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("bundle leaked %q", forbidden)
		}
	}
}

func TestBundleDoesNotOverwriteDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "support.zip")
	if err := os.WriteFile(destination, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(context.Background(), destination, "test", nil, paths.FromRoot(t.TempDir())); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "keep" {
		t.Fatalf("destination = %q, %v", data, err)
	}
}
