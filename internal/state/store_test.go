package state

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/spare-run/spare/internal/model"
)

func TestStorePersistsMachineInstanceAndEvents(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	emptyEvents, err := store.Events(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if emptyEvents == nil || len(emptyEvents) != 0 {
		t.Fatalf("expected a non-nil empty event list, got %#v", emptyEvents)
	}

	machine := model.Machine{ID: "spare_test", Hostname: "test", InitializedAt: now, LastProfiledAt: now}
	if err := store.SaveMachine(ctx, machine); err != nil {
		t.Fatal(err)
	}
	instance := model.Instance{
		ID:           model.RecipeSite,
		RecipeID:     model.RecipeSite,
		Mode:         model.ModeInstalled,
		DesiredState: model.DesiredRunning,
		Status:       model.StatusHealthy,
		RootPath:     "/tmp/site",
		Port:         7340,
		PortMode:     "auto",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := store.PutInstance(ctx, instance); err != nil {
		t.Fatal(err)
	}
	if err := store.AddEvent(ctx, model.Event{
		InstanceID: model.RecipeSite,
		Level:      "info",
		Kind:       "test",
		Message:    "Test event",
		Details:    map[string]any{"port": float64(7340)},
	}); err != nil {
		t.Fatal(err)
	}

	readMachine, err := store.Machine(ctx)
	if err != nil || readMachine.ID != machine.ID {
		t.Fatalf("machine round trip failed: %#v %v", readMachine, err)
	}
	readInstance, err := store.Instance(ctx, model.RecipeSite)
	if err != nil || readInstance.RootPath != instance.RootPath {
		t.Fatalf("instance round trip failed: %#v %v", readInstance, err)
	}
	events, err := store.Events(ctx, 10)
	if err != nil || len(events) != 1 || events[0].Kind != "test" {
		t.Fatalf("event round trip failed: %#v %v", events, err)
	}

	if err := store.DeleteInstance(ctx, model.RecipeSite); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Instance(ctx, model.RecipeSite); !IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestOpenRecoveringPreservesCorruptDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spare.db")
	original := []byte("not a sqlite database")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	store, recovered, err := OpenRecovering(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if recovered == nil || recovered.DatabasePath == "" {
		t.Fatal("corrupt database was not reported as recovered")
	}
	preserved, err := os.ReadFile(recovered.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(original) {
		t.Fatalf("preserved database = %q", preserved)
	}
	if _, err := store.Instances(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestQuarantinePreflightsEverySQLiteFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	path := filepath.Join(t.TempDir(), "spare.db")
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, path+"-wal"); err != nil {
		t.Fatal(err)
	}
	if _, err := quarantineDatabase(path, time.Now().UTC()); err == nil {
		t.Fatal("unsafe SQLite sidecar was accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("main database moved before sidecars were validated")
	}
}

func TestOpenRecoveringHandlesTruncatedSQLiteDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spare.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMachine(context.Background(), model.Machine{ID: "before-corruption"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, info.Size()/2); err != nil {
		t.Fatal(err)
	}

	recoveredStore, recovered, err := OpenRecovering(path)
	if err != nil {
		t.Fatal(err)
	}
	defer recoveredStore.Close()
	if recovered == nil {
		t.Fatal("truncated database was not preserved")
	}
}

func TestStorePublishesCommittedEvents(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := store.SubscribeEvents(ctx)
	if err := store.AddEvent(context.Background(), model.Event{
		Level:   "info",
		Kind:    "drop_file_received",
		Message: "report.pdf was received.",
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-events:
		if event.ID == 0 || event.Kind != "drop_file_received" ||
			event.Message != "report.pdf was received." {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for committed event")
	}
}

func TestMigrationRecordsCurrentSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var version string
	if err := database.QueryRow(`SELECT value FROM metadata WHERE key = 'schema_version'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "3" {
		t.Fatalf("schema version = %q", version)
	}
}

func TestJobProfileRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	profile := model.JobProfile{
		RecipeID: "downloads",
		Config:   map[string]any{"destination": "/tmp/downloads"},
		Port:     7344,
		PortMode: "fixed",
	}
	if err := store.PutJobProfile(context.Background(), profile); err != nil {
		t.Fatal(err)
	}
	stored, err := store.JobProfile(context.Background(), "downloads")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RecipeID != profile.RecipeID ||
		stored.Config["destination"] != "/tmp/downloads" ||
		stored.Port != 7344 ||
		stored.PortMode != "fixed" {
		t.Fatalf("profile = %#v", stored)
	}
}
