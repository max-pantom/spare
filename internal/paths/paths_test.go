package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestEndpointRoundTripIsPrivateAndLoopbackOnly(t *testing.T) {
	statePaths := FromRoot(t.TempDir())
	if err := statePaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	value := Endpoint{
		URL:       "http://127.0.0.1:7334",
		PID:       1234,
		StartedAt: time.Now().UTC(),
	}
	if err := statePaths.WriteEndpoint(value); err != nil {
		t.Fatal(err)
	}
	value.PID++
	if err := statePaths.WriteEndpoint(value); err != nil {
		t.Fatal(err)
	}
	loaded, err := statePaths.ReadEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.URL != value.URL || loaded.PID != value.PID {
		t.Fatalf("endpoint = %#v", loaded)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(statePaths.Endpoint)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("endpoint mode = %o", info.Mode().Perm())
		}
	}
}

func TestEndpointRejectsRemoteUnknownAndSymlinkedState(t *testing.T) {
	statePaths := FromRoot(t.TempDir())
	if err := statePaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"remote":   `{"url":"https://attacker.example","pid":12,"startedAt":"2026-01-01T00:00:00Z"}`,
		"unknown":  `{"url":"http://127.0.0.1:7331","pid":12,"startedAt":"2026-01-01T00:00:00Z","extra":true}`,
		"trailing": `{"url":"http://127.0.0.1:7331","pid":12,"startedAt":"2026-01-01T00:00:00Z"}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(statePaths.Endpoint, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := statePaths.ReadEndpoint(); err == nil {
				t.Fatal("expected unsafe endpoint state to be rejected")
			}
		})
	}
	if runtime.GOOS != "windows" {
		target := filepath.Join(statePaths.Root, "target")
		if err := os.WriteFile(target, []byte(`{"url":"http://127.0.0.1:7331","pid":12,"startedAt":"2026-01-01T00:00:00Z"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(statePaths.Endpoint); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, statePaths.Endpoint); err != nil {
			t.Fatal(err)
		}
		if _, err := statePaths.ReadEndpoint(); err == nil {
			t.Fatal("expected a symlinked endpoint to be rejected")
		}
	}
}

func TestEnsureRepairsPrivateDirectoryModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX directory modes")
	}
	statePaths := FromRoot(t.TempDir())
	if err := statePaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{statePaths.Root, statePaths.Logs, statePaths.JobPackages, statePaths.JobData} {
		if err := os.Chmod(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := statePaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{statePaths.Root, statePaths.Logs, statePaths.JobPackages, statePaths.JobData} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %o", directory, info.Mode().Perm())
		}
	}
}

func TestEnsureRejectsLinkedStateDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	statePaths := FromRoot(root)
	if err := os.Symlink(outside, statePaths.Logs); err != nil {
		t.Fatal(err)
	}
	if err := statePaths.Ensure(); err == nil {
		t.Fatal("Ensure accepted a linked logs directory")
	}
}
