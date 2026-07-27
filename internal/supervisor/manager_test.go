package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spare-run/spare/internal/health"
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

func TestPromoteTemporaryInstancePersistsIt(t *testing.T) {
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

	now := time.Now().UTC()
	manager.mu.Lock()
	manager.workers[model.RecipeDrop] = &worker{
		instance: model.Instance{
			ID:           model.RecipeDrop,
			RecipeID:     model.RecipeDrop,
			Mode:         model.ModeTemporary,
			DesiredState: model.DesiredRunning,
			Status:       model.StatusHealthy,
			Runtime:      "native",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		runtime:    &native.Driver{Executable: "unused"},
		leaseUntil: now.Add(leaseDuration),
	}
	manager.mu.Unlock()

	promoted, err := manager.Promote(model.RecipeDrop)
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Mode != model.ModeInstalled {
		t.Fatalf("mode = %q", promoted.Mode)
	}
	manager.mu.Lock()
	leaseUntil := manager.workers[model.RecipeDrop].leaseUntil
	manager.mu.Unlock()
	if !leaseUntil.IsZero() {
		t.Fatalf("lease remained after promotion: %s", leaseUntil)
	}
	stored, err := store.Instance(context.Background(), model.RecipeDrop)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Mode != model.ModeInstalled ||
		stored.DesiredState != model.DesiredRunning {
		t.Fatalf("stored instance = %#v", stored)
	}
	events, err := store.Events(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "instance_promoted" {
		t.Fatalf("promotion events = %#v", events)
	}
}

func TestConfigureStoppedInstancePersistsNewManifestConfiguration(t *testing.T) {
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

	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	now := time.Now().UTC()
	current := model.Instance{
		ID:           model.RecipeSite,
		RecipeID:     model.RecipeSite,
		Version:      "0.1.0",
		Runtime:      "native",
		Mode:         model.ModeInstalled,
		DesiredState: model.DesiredStopped,
		Status:       model.StatusStopped,
		RootPath:     oldRoot,
		DataPath:     oldRoot,
		Config:       map[string]any{"path": oldRoot},
		Port:         7340,
		PortMode:     "auto",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	manager.mu.Lock()
	manager.workers[current.ID] = &worker{
		instance: current,
		runtime:  &native.Driver{Executable: "unused"},
	}
	manager.mu.Unlock()
	if err := store.PutInstance(context.Background(), current); err != nil {
		t.Fatal(err)
	}

	updated, err := manager.Configure(current.ID, CreateRequest{
		RecipeID: model.RecipeSite,
		Config:   map[string]any{"path": newRoot},
		PortMode: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	resolvedNewRoot, err := filepath.EvalSymlinks(newRoot)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DataPath != resolvedNewRoot || updated.DesiredState != model.DesiredStopped ||
		updated.Status != model.StatusStopped {
		t.Fatalf("updated instance = %#v", updated)
	}
	stored, err := store.Instance(context.Background(), current.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DataPath != resolvedNewRoot {
		t.Fatalf("stored path = %q", stored.DataPath)
	}
	events, err := store.Events(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "instance_configured" {
		t.Fatalf("configuration events = %#v", events)
	}
	if _, err := os.Stat(oldRoot); err != nil {
		t.Fatalf("old selected folder changed: %v", err)
	}
}

func TestDropHealthChangeCreatesFileReceivedActivity(t *testing.T) {
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

	current := &worker{
		instance: model.Instance{
			ID:        model.RecipeDrop,
			RecipeID:  model.RecipeDrop,
			Mode:      model.ModeTemporary,
			Status:    model.StatusHealthy,
			ItemCount: 2,
		},
		healthyBefore: true,
	}
	manager.mu.Lock()
	manager.applyHealthSnapshotLocked(current, health.Snapshot{
		ItemCount:             3,
		LatestItem:            "brand-assets.zip",
		StorageAvailableBytes: 10_000,
	})
	manager.mu.Unlock()

	events, err := store.Events(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "drop_file_received" ||
		events[0].Message != "brand-assets.zip was received." ||
		events[0].Details["itemName"] != "brand-assets.zip" {
		t.Fatalf("file received events = %#v", events)
	}
}

func TestHookHealthChangeCreatesCapturedRequestActivity(t *testing.T) {
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

	current := &worker{
		instance: model.Instance{
			ID:        model.RecipeHook,
			RecipeID:  model.RecipeHook,
			Mode:      model.ModeTemporary,
			Status:    model.StatusHealthy,
			ItemCount: 1,
		},
		healthyBefore: true,
	}
	manager.mu.Lock()
	manager.applyHealthSnapshotLocked(current, health.Snapshot{
		ItemCount:  2,
		LatestItem: "POST /hook/stripe",
	})
	manager.mu.Unlock()

	events, err := store.Events(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "hook_request_captured" ||
		events[0].Message != "POST /hook/stripe was captured." ||
		events[0].Details["request"] != "POST /hook/stripe" {
		t.Fatalf("captured request events = %#v", events)
	}
}

func TestCopyDropFileKeepsDuplicateNamesAndContent(t *testing.T) {
	root := t.TempDir()
	sourceDirectory := t.TempDir()
	source := filepath.Join(sourceDirectory, "report.txt")
	if err := os.WriteFile(source, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := copyDropFile(root, source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := copyDropFile(root, source)
	if err != nil {
		t.Fatal(err)
	}
	if first != "report.txt" || second != "report (1).txt" {
		t.Fatalf("copied names = %q, %q", first, second)
	}
	firstData, err := os.ReadFile(filepath.Join(root, first))
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := os.ReadFile(filepath.Join(root, second))
	if err != nil {
		t.Fatal(err)
	}
	if string(firstData) != "first" || string(secondData) != "second" {
		t.Fatalf("copied content = %q, %q", firstData, secondData)
	}
}

func TestAddDropFilesValidatesEverySourceBeforeCopying(t *testing.T) {
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

	destination := t.TempDir()
	small := filepath.Join(t.TempDir(), "small.txt")
	large := filepath.Join(t.TempDir(), "large.txt")
	if err := os.WriteFile(small, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(large, []byte("too large"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	manager.mu.Lock()
	manager.workers[model.RecipeDrop] = &worker{
		instance: model.Instance{
			ID:           model.RecipeDrop,
			RecipeID:     model.RecipeDrop,
			Mode:         model.ModeTemporary,
			DesiredState: model.DesiredRunning,
			Status:       model.StatusHealthy,
			DataPath:     destination,
			Config:       map[string]any{"max-file-size": int64(4)},
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		runtime: &native.Driver{Executable: "unused"},
	}
	manager.mu.Unlock()

	if _, err := manager.AddDropFiles(model.RecipeDrop, []string{small, large}); err == nil {
		t.Fatal("expected the oversized file to reject the whole selection")
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation copied files before rejecting selection: %#v", entries)
	}

	names, err := manager.AddDropFiles(model.RecipeDrop, []string{small})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "small.txt" {
		t.Fatalf("added names = %#v", names)
	}
	current, err := manager.Get(model.RecipeDrop)
	if err != nil {
		t.Fatal(err)
	}
	if current.ItemCount != 1 {
		t.Fatalf("item count = %d", current.ItemCount)
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
