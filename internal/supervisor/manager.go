package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spare-run/spare/internal/backup"
	"github.com/spare-run/spare/internal/discovery"
	"github.com/spare-run/spare/internal/health"
	instancepkg "github.com/spare-run/spare/internal/instance"
	"github.com/spare-run/spare/internal/logs"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/network"
	"github.com/spare-run/spare/internal/recipe"
	spareRuntime "github.com/spare-run/spare/internal/runtime"
	"github.com/spare-run/spare/internal/state"
)

const (
	leaseDuration = 15 * time.Second
	healthEvery   = 10 * time.Second
)

type ManagerError struct {
	Code    string
	Message string
	Hint    string
}

func (e *ManagerError) Error() string {
	return e.Message
}

type CreateRequest = instancepkg.CreateRequest

type worker struct {
	instance      model.Instance
	process       spareRuntime.Process
	runtime       spareRuntime.Runtime
	log           io.WriteCloser
	mdns          io.Closer
	healthPort    int
	healthFails   int
	generation    uint64
	crashes       []time.Time
	leaseUntil    time.Time
	restartTimer  *time.Timer
	explicitStop  bool
	healthyBefore bool
}

type Manager struct {
	mu        sync.Mutex
	switchMu  sync.Mutex
	store     *state.Store
	logsDir   string
	machine   model.Machine
	registry  *recipe.Registry
	runtimes  map[string]spareRuntime.Runtime
	workers   map[string]*worker
	ctx       context.Context
	cancel    context.CancelFunc
	checker   health.Checker
	closed    bool
	waitDone  chan struct{}
	available func(string) bool
}

func New(
	store *state.Store,
	logsDir string,
	machine model.Machine,
	registry *recipe.Registry,
	runtimes map[string]spareRuntime.Runtime,
) (*Manager, error) {
	if registry == nil {
		return nil, errors.New("recipe registry is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		store:    store,
		logsDir:  logsDir,
		machine:  machine,
		registry: registry,
		runtimes: runtimes,
		workers:  map[string]*worker{},
		ctx:      ctx,
		cancel:   cancel,
		waitDone: make(chan struct{}),
	}
	instances, err := store.Instances(context.Background())
	if err != nil {
		cancel()
		return nil, err
	}
	for _, stored := range instances {
		migrated, migrateErr := manager.migrateInstance(stored)
		if migrateErr != nil {
			cancel()
			return nil, migrateErr
		}
		migrated.Status = model.StatusStopped
		driver, ok := runtimes[migrated.Runtime]
		if !ok {
			cancel()
			return nil, fmt.Errorf("runtime %q is unavailable for %s", migrated.Runtime, migrated.ID)
		}
		manager.workers[migrated.ID] = &worker{instance: migrated, runtime: driver}
		_ = store.PutInstance(context.Background(), migrated)
	}
	go manager.watchLeases()
	return manager, nil
}

func (m *Manager) migrateInstance(stored model.Instance) (model.Instance, error) {
	implementation, ok := m.registry.Get(stored.RecipeID)
	if !ok {
		return model.Instance{}, fmt.Errorf("installed recipe %q is unavailable", stored.RecipeID)
	}
	manifest := implementation.Manifest()
	if stored.Version == "" {
		stored.Version = manifest.Version
	}
	if stored.Runtime == "" {
		stored.Runtime = manifest.Runtime.Type
	}
	if stored.Config == nil {
		stored.Config = map[string]any{}
		if manifest.Storage.PathField != "" && stored.RootPath != "" {
			stored.Config[manifest.Storage.PathField] = stored.RootPath
		}
	}
	if stored.DataPath == "" {
		stored.DataPath = stored.RootPath
	}
	stored.StatePath = filepath.Join(filepath.Dir(m.logsDir), "jobs", stored.RecipeID)
	return stored, nil
}

func (m *Manager) Restore() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, current := range m.workers {
		if current.instance.Mode == model.ModeInstalled && current.instance.DesiredState == model.DesiredRunning {
			if err := m.launchLocked(current); err != nil {
				title := m.title(current.instance.RecipeID)
				m.failLocked(current, "worker_start_failed", "Unable to start "+title+".", "Run `spare doctor` and check the recipe logs.")
			}
		}
	}
}

func (m *Manager) Recipes() []model.Recipe {
	models := m.registry.Models(m.machine)
	m.mu.Lock()
	available := m.available
	m.mu.Unlock()
	if available == nil {
		return models
	}
	result := make([]model.Recipe, 0, len(models))
	for _, candidate := range models {
		if available(candidate.ID) {
			result = append(result, candidate)
		}
	}
	return result
}

func (m *Manager) SetRecipeAvailability(available func(string) bool) {
	m.mu.Lock()
	m.available = available
	m.mu.Unlock()
}

func (m *Manager) ExportBackup(id, destination string) error {
	m.mu.Lock()
	current, ok := m.workers[id]
	if !ok {
		m.mu.Unlock()
		return &ManagerError{
			Code:    "instance_not_found",
			Message: "The requested recipe is not installed.",
			Hint:    "Open Recipes to see the active job.",
		}
	}
	instance := current.instance
	m.mu.Unlock()
	if err := backup.ExportInstance(instance, destination); err != nil {
		return &ManagerError{
			Code:    "backup_export_failed",
			Message: "Spare could not create the backup.",
			Hint:    err.Error(),
		}
	}
	_ = m.store.AddEvent(context.Background(), model.Event{
		InstanceID: instance.ID,
		Level:      "info",
		Kind:       "backup_exported",
		Message:    m.title(instance.RecipeID) + " backup was exported.",
		Details:    map[string]any{"destination": destination},
		CreatedAt:  time.Now().UTC(),
	})
	return nil
}

func (m *Manager) RestoreBackup(source, destination string) (model.Instance, error) {
	manifest, err := backup.Inspect(source)
	if err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "backup_invalid",
			Message: "Spare could not read this backup.",
			Hint:    err.Error(),
		}
	}
	implementation, ok := m.registry.Get(manifest.RecipeID)
	if !ok {
		return model.Instance{}, &ManagerError{
			Code:    "unknown_recipe",
			Message: fmt.Sprintf("Recipe %q is not available.", manifest.RecipeID),
			Hint:    "Restore a backup made by one of this release's built-in recipes.",
		}
	}
	m.mu.Lock()
	hasInstance := len(m.workers) > 0
	m.mu.Unlock()
	if hasInstance {
		return model.Instance{}, &ManagerError{
			Code:    "role_already_exists",
			Message: "This computer already has an active job.",
			Hint:    "Remove the current job before restoring a backup.",
		}
	}
	if _, err := backup.Import(source, destination); err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "backup_restore_failed",
			Message: "Spare could not restore this backup.",
			Hint:    err.Error(),
		}
	}
	values := manifest.Config
	if values == nil {
		values = map[string]any{}
	}
	if pathField := implementation.Manifest().Storage.PathField; pathField != "" {
		values[pathField] = destination
	}
	instance, err := m.Create(CreateRequest{
		RecipeID: manifest.RecipeID,
		Mode:     model.ModeInstalled,
		Config:   values,
		Port:     manifest.Port,
		PortMode: manifest.PortMode,
	})
	if err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "backup_install_failed",
			Message: "The backup data was restored, but its job could not start.",
			Hint:    fmt.Sprintf("The restored files remain in %s. %v", destination, err),
		}
	}
	_ = m.store.AddEvent(context.Background(), model.Event{
		InstanceID: instance.ID,
		Level:      "info",
		Kind:       "backup_restored",
		Message:    m.title(instance.RecipeID) + " backup was restored.",
		Details:    map[string]any{"source": source, "destination": destination},
		CreatedAt:  time.Now().UTC(),
	})
	return instance, nil
}

func (m *Manager) AddDropFiles(id string, sources []string) ([]string, error) {
	if len(sources) == 0 || len(sources) > 100 {
		return nil, &ManagerError{
			Code:    "invalid_file_selection",
			Message: "Choose between 1 and 100 files.",
			Hint:    "Add fewer files and try again.",
		}
	}
	m.mu.Lock()
	current, ok := m.workers[id]
	if !ok || current.instance.RecipeID != model.RecipeDrop {
		m.mu.Unlock()
		return nil, &ManagerError{
			Code:    "drop_not_found",
			Message: "Drop is not active on this computer.",
			Hint:    "Set up Drop before adding files.",
		}
	}
	instance := current.instance
	m.mu.Unlock()
	root, err := filepath.EvalSymlinks(instance.DataPath)
	if err != nil {
		return nil, &ManagerError{
			Code:    "drop_folder_unavailable",
			Message: "Drop's selected folder is unavailable.",
			Hint:    "Reconnect the folder or configure Drop again.",
		}
	}
	maximumSize := configInt64(instance.Config["max-file-size"])
	resolvedSources := make([]string, 0, len(sources))
	for _, source := range sources {
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return nil, &ManagerError{
				Code:    "file_unavailable",
				Message: "Spare could not open " + filepath.Base(source) + ".",
				Hint:    err.Error(),
			}
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			return nil, &ManagerError{
				Code:    "invalid_file_selection",
				Message: filepath.Base(source) + " is not a regular file.",
				Hint:    "Choose files rather than folders or special devices.",
			}
		}
		if maximumSize > 0 && info.Size() > maximumSize {
			return nil, &ManagerError{
				Code:    "file_too_large",
				Message: filepath.Base(source) + " is larger than Drop's file limit.",
				Hint:    "Choose a smaller file or reconfigure Drop with a larger limit.",
			}
		}
		resolvedSources = append(resolvedSources, resolved)
	}
	added := make([]string, 0, len(resolvedSources))
	for _, resolved := range resolvedSources {
		name, err := copyDropFile(root, resolved)
		if err != nil {
			m.recordDropFilesAdded(instance.ID, added)
			return added, &ManagerError{
				Code:    "file_copy_failed",
				Message: "Spare could not add " + filepath.Base(resolved) + " to Drop.",
				Hint:    err.Error(),
			}
		}
		added = append(added, name)
	}
	m.recordDropFilesAdded(instance.ID, added)
	return added, nil
}

func (m *Manager) recordDropFilesAdded(id string, added []string) {
	if len(added) == 0 {
		return
	}
	m.mu.Lock()
	if current, ok := m.workers[id]; ok {
		current.instance.ItemCount += len(added)
		current.instance.UpdatedAt = time.Now().UTC()
		m.persistLocked(current)
	}
	m.mu.Unlock()
	_ = m.store.AddEvent(context.Background(), model.Event{
		InstanceID: id,
		Level:      "info",
		Kind:       "drop_files_added",
		Message:    fmt.Sprintf("%d %s added to Drop.", len(added), plural(len(added), "file", "files")),
		Details:    map[string]any{"count": len(added), "names": added},
		CreatedAt:  time.Now().UTC(),
	})
}

func (m *Manager) Create(request CreateRequest) (model.Instance, error) {
	m.mu.Lock()
	available := m.available
	m.mu.Unlock()
	if available != nil && !available(request.RecipeID) {
		return model.Instance{}, &ManagerError{
			Code:    "job_not_installed",
			Message: "This optional job is not installed.",
			Hint:    "Download and install the job package before starting it.",
		}
	}
	implementation, ok := m.registry.Get(request.RecipeID)
	if !ok {
		return model.Instance{}, &ManagerError{
			Code:    "unknown_recipe",
			Message: fmt.Sprintf("Recipe %q is not available.", request.RecipeID),
			Hint:    "Run `spare recipe list` to see built-in recipes.",
		}
	}
	manifest := implementation.Manifest()
	compatibility := manifest.Compatible(m.machine)
	if !compatibility.Supported {
		return model.Instance{}, &ManagerError{
			Code:    "recipe_not_supported",
			Message: manifest.Name + " is not supported on this computer.",
			Hint:    strings.Join(compatibility.Reasons, " "),
		}
	}
	candidate, err := instancepkg.Build(m.registry, request)
	if err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "invalid_recipe_configuration",
			Message: "The " + manifest.Name + " configuration is invalid.",
			Hint:    err.Error(),
		}
	}
	candidate.StatePath = filepath.Join(filepath.Dir(m.logsDir), "jobs", candidate.RecipeID)
	if err := os.MkdirAll(candidate.StatePath, 0o700); err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "job_storage_unavailable",
			Message: "Spare could not prepare private storage for " + manifest.Name + ".",
			Hint:    err.Error(),
		}
	}
	if err := os.Chmod(candidate.StatePath, 0o700); err != nil {
		return model.Instance{}, err
	}

	m.mu.Lock()
	if existing := m.onlyWorkerLocked(); existing != nil {
		if instancepkg.SameConfiguration(existing.instance, request, candidate) {
			result := m.decorate(existing.instance)
			m.mu.Unlock()
			return result, nil
		}
		existingTitle := m.title(existing.instance.RecipeID)
		m.mu.Unlock()
		return model.Instance{}, &ManagerError{
			Code:    "role_already_exists",
			Message: "This computer is already a " + existingTitle + ".",
			Hint:    "Remove the current role before installing a different recipe or configuration.",
		}
	}
	m.mu.Unlock()

	port, err := network.SelectPort(request.Port, candidate.PortMode)
	if err != nil {
		return model.Instance{}, managerNetworkError(err)
	}
	candidate.Port = port
	driver, ok := m.runtimes[candidate.Runtime]
	if !ok {
		return model.Instance{}, &ManagerError{
			Code:    "runtime_unavailable",
			Message: fmt.Sprintf("The %s runtime is unavailable.", candidate.Runtime),
			Hint:    "Reinstall Spare and try again.",
		}
	}
	if err := driver.Prepare(context.Background(), candidate); err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "runtime_prepare_failed",
			Message: "Unable to prepare " + manifest.Name + ".",
			Hint:    err.Error(),
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return model.Instance{}, &ManagerError{Code: "daemon_stopping", Message: "Spare is stopping.", Hint: "Try again in a moment."}
	}
	if existing := m.onlyWorkerLocked(); existing != nil {
		if instancepkg.SameConfiguration(existing.instance, request, candidate) {
			return m.decorate(existing.instance), nil
		}
		return model.Instance{}, &ManagerError{
			Code:    "role_already_exists",
			Message: "This computer already has a role.",
			Hint:    "Remove the current role before installing another one.",
		}
	}

	current := &worker{instance: candidate, runtime: driver}
	if request.Mode == model.ModeTemporary {
		current.leaseUntil = time.Now().Add(leaseDuration)
	}
	m.workers[candidate.ID] = current
	if request.Mode == model.ModeInstalled {
		if err := m.store.PutInstance(context.Background(), candidate); err != nil {
			delete(m.workers, candidate.ID)
			return model.Instance{}, err
		}
	}
	if err := m.launchLocked(current); err != nil {
		delete(m.workers, candidate.ID)
		if request.Mode == model.ModeInstalled {
			_ = m.store.DeleteInstance(context.Background(), candidate.ID)
		}
		return model.Instance{}, &ManagerError{
			Code:    "worker_start_failed",
			Message: "Unable to start " + manifest.Name + ".",
			Hint:    err.Error(),
		}
	}
	_ = m.store.PutJobProfile(context.Background(), profileFor(candidate))
	m.eventLocked(current, "info", "instance_created", manifest.Name+" started.", map[string]any{
		"mode": request.Mode,
		"port": port,
	})
	return m.decorate(current.instance), nil
}

func (m *Manager) List() []model.Instance {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]model.Instance, 0, len(m.workers))
	for _, current := range m.workers {
		result = append(result, m.decorate(current.instance))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (m *Manager) Get(id string) (model.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok {
		return model.Instance{}, instanceNotFound(id)
	}
	return m.decorate(current.instance), nil
}

func (m *Manager) Configure(id string, request CreateRequest) (model.Instance, error) {
	m.mu.Lock()
	current, ok := m.workers[id]
	if !ok {
		m.mu.Unlock()
		return model.Instance{}, instanceNotFound(id)
	}
	if request.RecipeID == "" {
		request.RecipeID = current.instance.RecipeID
	}
	if request.RecipeID != current.instance.RecipeID {
		m.mu.Unlock()
		return model.Instance{}, &ManagerError{
			Code:    "recipe_change_requires_removal",
			Message: "Configuration cannot change this job into another recipe.",
			Hint:    "Remove the current job, then set up the other recipe.",
		}
	}
	request.Mode = current.instance.Mode
	oldInstance := current.instance
	oldRuntime := current.runtime
	m.mu.Unlock()

	candidate, err := instancepkg.Build(m.registry, request)
	if err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "invalid_recipe_configuration",
			Message: "The " + m.title(request.RecipeID) + " configuration is invalid.",
			Hint:    err.Error(),
		}
	}
	candidate.StatePath = oldInstance.StatePath
	driver, ok := m.runtimes[candidate.Runtime]
	if !ok {
		return model.Instance{}, &ManagerError{
			Code:    "runtime_unavailable",
			Message: fmt.Sprintf("The %s runtime is unavailable.", candidate.Runtime),
			Hint:    "Reinstall Spare and try again.",
		}
	}
	if err := driver.Prepare(context.Background(), candidate); err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "runtime_prepare_failed",
			Message: "Unable to prepare " + m.title(candidate.RecipeID) + ".",
			Hint:    err.Error(),
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok = m.workers[id]
	if !ok || current.instance.UpdatedAt != oldInstance.UpdatedAt {
		return model.Instance{}, &ManagerError{
			Code:    "instance_changed",
			Message: "The active job changed while its settings were open.",
			Hint:    "Review the current settings and try again.",
		}
	}
	wasRunning := oldInstance.DesiredState == model.DesiredRunning
	m.stopProcessLocked(current)
	port := request.Port
	if candidate.PortMode == "auto" && oldInstance.PortMode == "auto" &&
		network.PortAvailable(oldInstance.Port) {
		port = oldInstance.Port
	} else {
		port, err = network.SelectPort(request.Port, candidate.PortMode)
		if err != nil {
			if wasRunning {
				_ = m.launchLocked(current)
			}
			return model.Instance{}, managerNetworkError(err)
		}
	}
	candidate.Port = port
	candidate.CreatedAt = oldInstance.CreatedAt
	candidate.DesiredState = oldInstance.DesiredState
	candidate.Status = model.StatusStopped
	candidate.UpdatedAt = time.Now().UTC()
	current.instance = candidate
	current.runtime = driver
	current.explicitStop = !wasRunning
	current.crashes = nil
	if candidate.Mode == model.ModeTemporary {
		current.leaseUntil = time.Now().Add(leaseDuration)
	}
	if wasRunning {
		if err := m.launchLocked(current); err != nil {
			current.instance = oldInstance
			current.runtime = oldRuntime
			current.explicitStop = false
			_ = m.launchLocked(current)
			return model.Instance{}, &ManagerError{
				Code:    "configuration_start_failed",
				Message: "Spare could not start the new configuration.",
				Hint:    err.Error() + " The previous configuration was restored.",
			}
		}
	} else {
		m.persistLocked(current)
	}
	m.eventLocked(current, "info", "instance_configured", m.title(candidate.RecipeID)+" configuration was updated.", map[string]any{
		"port": port,
	})
	_ = m.store.PutJobProfile(context.Background(), profileFor(candidate))
	return m.decorate(current.instance), nil
}

func (m *Manager) Start(id string) (model.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok {
		return model.Instance{}, instanceNotFound(id)
	}
	if current.process != nil && current.instance.DesiredState == model.DesiredRunning {
		return m.decorate(current.instance), nil
	}
	current.instance.DesiredState = model.DesiredRunning
	current.explicitStop = false
	current.crashes = nil
	if current.instance.Mode == model.ModeTemporary {
		current.leaseUntil = time.Now().Add(leaseDuration)
	}
	if err := m.launchLocked(current); err != nil {
		title := m.title(current.instance.RecipeID)
		m.failLocked(current, "worker_start_failed", "Unable to start "+title+".", err.Error())
		return model.Instance{}, err
	}
	m.eventLocked(current, "info", "instance_started", m.title(current.instance.RecipeID)+" started.", nil)
	return m.decorate(current.instance), nil
}

func (m *Manager) Stop(id string) (model.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok {
		return model.Instance{}, instanceNotFound(id)
	}
	if current.instance.DesiredState == model.DesiredStopped && current.process == nil {
		return m.decorate(current.instance), nil
	}
	current.instance.DesiredState = model.DesiredStopped
	current.explicitStop = true
	m.stopProcessLocked(current)
	current.instance.Status = model.StatusStopped
	current.instance.Problem = nil
	current.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(current)
	m.eventLocked(current, "info", "instance_stopped", m.title(current.instance.RecipeID)+" stopped.", nil)
	return m.decorate(current.instance), nil
}

func (m *Manager) Heartbeat(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok || current.instance.Mode != model.ModeTemporary {
		return &ManagerError{Code: "temporary_instance_not_found", Message: "The temporary recipe is no longer running."}
	}
	current.leaseUntil = time.Now().Add(leaseDuration)
	return nil
}

func (m *Manager) Promote(id string) (model.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok || current.instance.Mode != model.ModeTemporary {
		return model.Instance{}, &ManagerError{
			Code:    "temporary_instance_not_found",
			Message: "The temporary recipe is no longer running.",
			Hint:    "Choose the recipe again and select Keep running after login.",
		}
	}
	current.instance.Mode = model.ModeInstalled
	current.leaseUntil = time.Time{}
	current.instance.UpdatedAt = time.Now().UTC()
	if err := m.store.PutInstance(context.Background(), current.instance); err != nil {
		current.instance.Mode = model.ModeTemporary
		current.leaseUntil = time.Now().Add(leaseDuration)
		return model.Instance{}, err
	}
	m.eventLocked(current, "info", "instance_promoted", m.title(current.instance.RecipeID)+" will keep running after login.", nil)
	return m.decorate(current.instance), nil
}

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok {
		return instanceNotFound(id)
	}
	title := m.title(current.instance.RecipeID)
	_ = m.store.PutJobProfile(context.Background(), profileFor(current.instance))
	current.instance.Status = model.StatusRemoving
	current.instance.DesiredState = model.DesiredStopped
	current.explicitStop = true
	m.stopProcessLocked(current)
	if err := current.runtime.Remove(context.Background(), current.instance); err != nil {
		return err
	}
	if current.instance.Mode == model.ModeInstalled {
		if err := m.store.DeleteInstance(context.Background(), id); err != nil && !state.IsNotFound(err) {
			return err
		}
	}
	delete(m.workers, id)
	removeLogs(m.logsDir, id)
	_ = m.store.AddEvent(context.Background(), model.Event{
		InstanceID: id,
		Level:      "info",
		Kind:       "instance_removed",
		Message:    title + " was removed. Its selected folder was left unchanged.",
	})
	return nil
}

// Switch replaces the single active job and restores the previous job if the
// replacement cannot start. Saved profiles survive both paths.
func (m *Manager) Switch(request CreateRequest) (model.Instance, error) {
	m.switchMu.Lock()
	defer m.switchMu.Unlock()
	current := m.List()
	if len(current) == 0 {
		return m.Create(request)
	}
	previous := current[0]
	if previous.RecipeID == request.RecipeID {
		return m.Configure(previous.ID, request)
	}
	previousRequest := CreateRequest{
		RecipeID: previous.RecipeID,
		Mode:     previous.Mode,
		Config:   previous.Config,
		Port:     previous.Port,
		PortMode: previous.PortMode,
	}
	if err := m.Remove(previous.ID); err != nil {
		return model.Instance{}, err
	}
	next, err := m.Create(request)
	if err == nil {
		return next, nil
	}
	if _, rollbackErr := m.Create(previousRequest); rollbackErr != nil {
		return model.Instance{}, &ManagerError{
			Code:    "job_switch_and_rollback_failed",
			Message: "The new job could not start, and Spare could not restore the previous job.",
			Hint:    err.Error() + " Restore error: " + rollbackErr.Error(),
		}
	}
	return model.Instance{}, &ManagerError{
		Code:    "job_switch_failed",
		Message: "The new job could not start.",
		Hint:    err.Error() + " The previous job was restored.",
	}
}

func (m *Manager) Shutdown() {
	m.cancel()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	for _, current := range m.workers {
		if current.restartTimer != nil {
			current.restartTimer.Stop()
		}
		m.stopProcessLocked(current)
	}
	m.mu.Unlock()
	select {
	case <-m.waitDone:
	case <-time.After(2 * time.Second):
	}
}

func (m *Manager) launchLocked(current *worker) error {
	if current.process != nil {
		return nil
	}
	port := current.instance.Port
	if !network.PortAvailable(port) {
		if current.instance.PortMode == "auto" {
			selected, err := network.SelectPort(0, "auto")
			if err != nil {
				return err
			}
			port = selected
			current.instance.Port = selected
			m.eventLocked(current, "warning", "port_changed", m.title(current.instance.RecipeID)+" moved to a free local port.", map[string]any{"port": selected})
		} else {
			return &ManagerError{
				Code:    "port_in_use",
				Message: fmt.Sprintf("Port %d is already in use.", port),
				Hint:    "Remove and reinstall the recipe with `--port auto` or another port.",
			}
		}
	}
	healthPort, err := network.FreeLoopbackPort()
	if err != nil {
		return err
	}
	logWriter, err := logs.NewRotatingWriter(filepath.Join(m.logsDir, current.instance.ID+".log"), 5*1024*1024, 5)
	if err != nil {
		return err
	}
	process, err := current.runtime.Start(m.ctx, current.instance, healthPort, logWriter, logWriter)
	if err != nil {
		_ = logWriter.Close()
		return err
	}

	current.generation++
	generation := current.generation
	current.process = process
	current.log = logWriter
	current.healthPort = healthPort
	current.healthFails = 0
	current.healthyBefore = false
	current.instance.Status = model.StatusStarting
	current.instance.Problem = nil
	current.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(current)

	go m.wait(current.instance.ID, generation, process)
	go m.monitor(current.instance.ID, generation, healthPort)
	return nil
}

func (m *Manager) wait(id string, generation uint64, process spareRuntime.Process) {
	err := process.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok || current.generation != generation || current.process != process {
		return
	}
	if current.log != nil {
		_ = current.log.Close()
		current.log = nil
	}
	current.process = nil
	m.stopMDNSLocked(current)

	if current.instance.DesiredState == model.DesiredStopped || current.explicitStop || m.closed {
		current.instance.Status = model.StatusStopped
		current.instance.UpdatedAt = time.Now().UTC()
		m.persistLocked(current)
		return
	}
	if current.instance.Mode == model.ModeTemporary && time.Now().After(current.leaseUntil) {
		delete(m.workers, id)
		return
	}

	now := time.Now()
	current.crashes = append(recentCrashes(current.crashes, now, 5*time.Minute), now)
	title := m.title(current.instance.RecipeID)
	if len(current.crashes) >= 5 {
		m.failLocked(current, "restart_limit_reached", title+" stopped after repeatedly failing.", "Check the recipe logs, then start it again after fixing the problem.")
		return
	}

	delay := restartDelay(len(current.crashes))
	message := title + " stopped unexpectedly. Spare will restart it."
	if err != nil {
		message = fmt.Sprintf("%s stopped unexpectedly (%s). Spare will restart it.", title, err)
	}
	current.instance.Status = model.StatusDegraded
	current.instance.Problem = &model.Problem{
		Code:     "worker_exited",
		Severity: "warning",
		Summary:  message,
		Recovery: "Spare is restarting the recipe automatically.",
	}
	current.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(current)
	m.eventLocked(current, "warning", "worker_exited", message, map[string]any{"restartInSeconds": int(delay.Seconds())})
	current.restartTimer = time.AfterFunc(delay, func() {
		m.restart(id, generation)
	})
}

func (m *Manager) restart(id string, generation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok || current.generation != generation || current.process != nil ||
		current.instance.DesiredState != model.DesiredRunning || m.closed {
		return
	}
	if current.instance.Mode == model.ModeTemporary && time.Now().After(current.leaseUntil) {
		delete(m.workers, id)
		return
	}
	if err := m.launchLocked(current); err != nil {
		m.failLocked(current, "worker_restart_failed", "Unable to restart "+m.title(current.instance.RecipeID)+".", err.Error())
	}
}

func (m *Manager) monitor(id string, generation uint64, healthPort int) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-timer.C:
			timer.Reset(healthEvery)
			checkContext, cancel := context.WithTimeout(m.ctx, 2*time.Second)
			snapshot, err := m.checker.Check(checkContext, healthPort)
			cancel()
			m.mu.Lock()
			current, ok := m.workers[id]
			if !ok || current.generation != generation || current.process == nil {
				m.mu.Unlock()
				return
			}
			if err == nil {
				m.applyHealthSnapshotLocked(current, snapshot)
				m.mu.Unlock()
				continue
			}
			current.healthFails++
			if current.healthFails >= 3 {
				title := m.title(current.instance.RecipeID)
				current.instance.Status = model.StatusDegraded
				current.instance.Problem = &model.Problem{
					Code:     "health_check_failed",
					Severity: "warning",
					Summary:  title + " stopped responding.",
					Recovery: "Spare is restarting the recipe automatically.",
				}
				current.instance.UpdatedAt = time.Now().UTC()
				m.persistLocked(current)
				_ = current.runtime.Stop(context.Background(), current.instance, current.process)
				m.mu.Unlock()
				return
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) applyHealthSnapshotLocked(current *worker, snapshot health.Snapshot) {
	previousItemCount := current.instance.ItemCount
	current.healthFails = 0
	current.instance.StorageAvailableBytes = snapshot.StorageAvailableBytes
	current.instance.ItemCount = snapshot.ItemCount
	snapshotStatus := snapshot.Status
	if snapshotStatus == "" {
		snapshotStatus = model.StatusHealthy
	}
	if snapshotStatus != model.StatusHealthy {
		code := snapshot.ProblemCode
		if code == "" {
			code = "worker_reported_degraded"
		}
		summary := snapshot.ProblemSummary
		if summary == "" {
			summary = m.title(current.instance.RecipeID) + " needs attention."
		}
		recovery := snapshot.ProblemRecovery
		if recovery == "" {
			recovery = "Run `spare doctor` for more information."
		}
		changed := current.instance.Status != model.StatusDegraded ||
			current.instance.Problem == nil ||
			current.instance.Problem.Code != code ||
			current.instance.Problem.Summary != summary ||
			current.instance.Problem.Recovery != recovery
		current.instance.Status = model.StatusDegraded
		current.instance.Problem = &model.Problem{
			Code:     code,
			Severity: "warning",
			Summary:  summary,
			Recovery: recovery,
		}
		if changed {
			current.instance.UpdatedAt = time.Now().UTC()
			m.persistLocked(current)
			m.eventLocked(current, "warning", "instance_degraded", summary, map[string]any{"code": code})
		}
		return
	}
	if current.healthyBefore && current.instance.RecipeID == model.RecipeDrop &&
		snapshot.ItemCount > previousItemCount {
		count := snapshot.ItemCount - previousItemCount
		message := "Drop received a file."
		if snapshot.LatestItem != "" {
			message = snapshot.LatestItem + " was received."
		}
		if count > 1 {
			message = fmt.Sprintf("Drop received %d files.", count)
		}
		m.eventLocked(current, "info", "drop_file_received", message, map[string]any{
			"count":    count,
			"itemName": snapshot.LatestItem,
		})
	}
	if current.healthyBefore && current.instance.RecipeID == model.RecipeHook &&
		snapshot.ItemCount > previousItemCount {
		count := snapshot.ItemCount - previousItemCount
		message := "Hook captured a request."
		if snapshot.LatestItem != "" {
			message = snapshot.LatestItem + " was captured."
		}
		if count > 1 {
			message = fmt.Sprintf("Hook captured %d requests.", count)
		}
		m.eventLocked(current, "info", "hook_request_captured", message, map[string]any{
			"count":   count,
			"request": snapshot.LatestItem,
		})
	}
	if !current.healthyBefore || current.instance.Status != model.StatusHealthy || current.instance.Problem != nil {
		now := time.Now().UTC()
		recovered := current.healthyBefore
		current.healthyBefore = true
		current.instance.Status = model.StatusHealthy
		current.instance.Problem = nil
		current.instance.StartedAt = &now
		current.instance.UpdatedAt = now
		m.persistLocked(current)
		if current.mdns == nil {
			current.mdns, _ = discovery.Advertise(m.title(current.instance.RecipeID), m.machine.Hostname, m.machine.ID, current.instance.Port)
		}
		kind := "instance_healthy"
		message := m.title(current.instance.RecipeID) + " is ready."
		if recovered {
			kind = "instance_recovered"
			message = m.title(current.instance.RecipeID) + " recovered."
		}
		m.eventLocked(current, "info", kind, message, nil)
	}
}

func (m *Manager) watchLeases() {
	defer close(m.waitDone)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.expireLeases(time.Now())
		}
	}
}

func (m *Manager) expireLeases(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, current := range m.workers {
		if current.instance.Mode != model.ModeTemporary || now.Before(current.leaseUntil) {
			continue
		}
		title := m.title(current.instance.RecipeID)
		current.instance.DesiredState = model.DesiredStopped
		current.explicitStop = true
		m.stopProcessLocked(current)
		delete(m.workers, id)
		_ = m.store.AddEvent(context.Background(), model.Event{
			InstanceID: id,
			Level:      "info",
			Kind:       "temporary_instance_expired",
			Message:    "Temporary " + title + " stopped after its lease owner closed.",
		})
	}
}

func (m *Manager) stopProcessLocked(current *worker) {
	if current.restartTimer != nil {
		current.restartTimer.Stop()
		current.restartTimer = nil
	}
	m.stopMDNSLocked(current)
	if current.process != nil {
		current.generation++
		process := current.process
		current.process = nil
		_ = current.runtime.Stop(context.Background(), current.instance, process)
	}
	if current.log != nil {
		_ = current.log.Close()
		current.log = nil
	}
}

func (m *Manager) stopMDNSLocked(current *worker) {
	if current.mdns != nil {
		_ = current.mdns.Close()
		current.mdns = nil
	}
}

func (m *Manager) failLocked(current *worker, code, summary, recovery string) {
	current.instance.Status = model.StatusFailed
	current.instance.Problem = &model.Problem{
		Code:     code,
		Severity: "error",
		Summary:  summary,
		Recovery: recovery,
	}
	current.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(current)
	m.eventLocked(current, "error", code, summary, nil)
}

func (m *Manager) persistLocked(current *worker) {
	if current.instance.Mode == model.ModeInstalled {
		_ = m.store.PutInstance(context.Background(), current.instance)
	}
}

func (m *Manager) eventLocked(current *worker, level, kind, message string, details map[string]any) {
	_ = m.store.AddEvent(context.Background(), model.Event{
		InstanceID: current.instance.ID,
		Level:      level,
		Kind:       kind,
		Message:    message,
		Details:    details,
	})
}

func (m *Manager) decorate(instance model.Instance) model.Instance {
	instance.URLs = network.URLs(network.Endpoints(m.machine.Hostname, instance.Port))
	return instance
}

func (m *Manager) title(id string) string {
	if implementation, ok := m.registry.Get(id); ok {
		return implementation.Manifest().Name
	}
	return id
}

func (m *Manager) onlyWorkerLocked() *worker {
	for _, current := range m.workers {
		return current
	}
	return nil
}

func managerNetworkError(err error) error {
	var networkError *network.Error
	if errors.As(err, &networkError) {
		return &ManagerError{
			Code:    networkError.Code,
			Message: networkError.Message,
			Hint:    networkError.Hint,
		}
	}
	return err
}

func instanceNotFound(id string) *ManagerError {
	return &ManagerError{
		Code:    "instance_not_found",
		Message: fmt.Sprintf("Recipe %q is not installed.", id),
		Hint:    "Run `spare recipe list` to see available recipes.",
	}
}

func removeLogs(logsDir, id string) {
	logPath := filepath.Join(logsDir, id+".log")
	_ = os.Remove(logPath)
	for index := 1; index <= 5; index++ {
		_ = os.Remove(fmt.Sprintf("%s.%d", logPath, index))
	}
}

func profileFor(instance model.Instance) model.JobProfile {
	return model.JobProfile{
		RecipeID:  instance.RecipeID,
		Config:    instance.Config,
		Port:      instance.Port,
		PortMode:  instance.PortMode,
		UpdatedAt: time.Now().UTC(),
	}
}

func recentCrashes(crashes []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	result := make([]time.Time, 0, len(crashes))
	for _, crash := range crashes {
		if crash.After(cutoff) {
			result = append(result, crash)
		}
	}
	return result
}

func restartDelay(crashCount int) time.Duration {
	if crashCount < 1 {
		crashCount = 1
	}
	delay := time.Second << (crashCount - 1)
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func configInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	default:
		return 0
	}
}

func copyDropFile(root, source string) (string, error) {
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	temporary, err := os.CreateTemp(root, ".spare-drop-*")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := io.Copy(temporary, input); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}

	base := filepath.Base(source)
	extension := filepath.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	for suffix := 0; suffix < 10_000; suffix++ {
		name := base
		if suffix > 0 {
			name = fmt.Sprintf("%s (%d)%s", stem, suffix, extension)
		}
		target := filepath.Join(root, name)
		if err := os.Link(temporaryPath, target); err == nil {
			return name, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", errors.New("too many files use this name")
}

func plural(count int, singular, multiple string) string {
	if count == 1 {
		return singular
	}
	return multiple
}
