package supervisor

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/recipes"
	spareRuntime "github.com/spare-run/spare/internal/runtime"
	"github.com/spare-run/spare/internal/runtime/native"
	"github.com/spare-run/spare/internal/state"
)

func TestTemporaryLeaseExpires(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	registry, err := recipes.Builtins()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(
		store,
		t.TempDir(),
		model.Machine{ID: "spare_test", Hostname: "test"},
		registry,
		map[string]spareRuntime.Runtime{"native": &native.Driver{Executable: "unused"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()

	manager.mu.Lock()
	manager.workers[model.RecipeSite] = &worker{
		instance: model.Instance{
			ID:       model.RecipeSite,
			RecipeID: model.RecipeSite,
			Mode:     model.ModeTemporary,
			Status:   model.StatusHealthy,
		},
		leaseUntil: time.Now().Add(-time.Second),
	}
	manager.mu.Unlock()
	manager.expireLeases(time.Now())

	if _, err := manager.Get(model.RecipeSite); err == nil {
		t.Fatal("expected the temporary Site to expire")
	}
	events, err := store.Events(context.Background(), 10)
	if err != nil || len(events) != 1 || events[0].Kind != "temporary_instance_expired" {
		t.Fatalf("expected expiration event, got %#v %v", events, err)
	}
}

func TestRestartBackoffAndWindow(t *testing.T) {
	if delay := restartDelay(1); delay != time.Second {
		t.Fatalf("first delay = %s", delay)
	}
	if delay := restartDelay(10); delay != 30*time.Second {
		t.Fatalf("capped delay = %s", delay)
	}
	now := time.Now()
	recent := recentCrashes([]time.Time{
		now.Add(-10 * time.Minute),
		now.Add(-time.Minute),
	}, now, 5*time.Minute)
	if len(recent) != 1 {
		t.Fatalf("expected one recent crash, got %d", len(recent))
	}
}

func TestManagerMigratesLegacySiteInstance(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	legacy := model.Instance{
		ID:           model.RecipeSite,
		RecipeID:     model.RecipeSite,
		Mode:         model.ModeInstalled,
		DesiredState: model.DesiredStopped,
		Status:       model.StatusStopped,
		RootPath:     t.TempDir(),
		Port:         7340,
		PortMode:     "auto",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.PutInstance(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}
	registry, err := recipes.Builtins()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(
		store,
		t.TempDir(),
		model.Machine{ID: "spare_test", Hostname: "test"},
		registry,
		map[string]spareRuntime.Runtime{"native": &native.Driver{Executable: "unused"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()
	migrated, err := manager.Get(model.RecipeSite)
	if err != nil {
		t.Fatal(err)
	}
	if migrated.Version != "0.1.0" || migrated.Runtime != "native" ||
		migrated.DataPath != legacy.RootPath || migrated.Config["path"] != legacy.RootPath {
		t.Fatalf("legacy instance was not migrated: %#v", migrated)
	}
}
