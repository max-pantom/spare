package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
	mu       sync.Mutex
	store    *state.Store
	logsDir  string
	machine  model.Machine
	registry *recipe.Registry
	runtimes map[string]spareRuntime.Runtime
	workers  map[string]*worker
	ctx      context.Context
	cancel   context.CancelFunc
	checker  health.Checker
	closed   bool
	waitDone chan struct{}
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
	return m.registry.Models(m.machine)
}

func (m *Manager) Create(request CreateRequest) (model.Instance, error) {
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

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.workers[id]
	if !ok {
		return instanceNotFound(id)
	}
	title := m.title(current.instance.RecipeID)
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
				current.healthFails = 0
				current.instance.StorageAvailableBytes = snapshot.StorageAvailableBytes
				current.instance.ItemCount = snapshot.ItemCount
				if !current.healthyBefore {
					now := time.Now().UTC()
					current.healthyBefore = true
					current.instance.Status = model.StatusHealthy
					current.instance.Problem = nil
					current.instance.StartedAt = &now
					current.instance.UpdatedAt = now
					m.persistLocked(current)
					if current.mdns == nil {
						current.mdns, _ = discovery.Advertise(m.machine.Hostname, current.instance.Port)
					}
					m.eventLocked(current, "info", "instance_healthy", m.title(current.instance.RecipeID)+" is ready.", nil)
				}
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
			Message:    "Temporary " + title + " stopped after its terminal closed.",
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
