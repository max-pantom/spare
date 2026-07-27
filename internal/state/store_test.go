package state

import (
	"context"
	"database/sql"
	"path/filepath"
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
	if version != "2" {
		t.Fatalf("schema version = %q", version)
	}
}
