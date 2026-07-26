package supervisor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spare-run/spare/internal/discovery"
	"github.com/spare-run/spare/internal/logs"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/profile"
	"github.com/spare-run/spare/internal/site"
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

type CreateRequest struct {
	Mode     string
	RootPath string
	Port     int
	PortMode string
}

type worker struct {
	instance      model.Instance
	cmd           *exec.Cmd
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
	exe      string
	machine  model.Machine
	workers  map[string]*worker
	ctx      context.Context
	cancel   context.CancelFunc
	http     *http.Client
	closed   bool
	waitDone chan struct{}
}

func New(store *state.Store, logsDir, executable string, machine model.Machine) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		store:   store,
		logsDir: logsDir,
		exe:     executable,
		machine: machine,
		workers: map[string]*worker{},
		ctx:     ctx,
		cancel:  cancel,
		http: &http.Client{
			Timeout: 2 * time.Second,
		},
		waitDone: make(chan struct{}),
	}
	instances, err := store.Instances(context.Background())
	if err != nil {
		cancel()
		return nil, err
	}
	for _, instance := range instances {
		instance.Status = model.StatusStopped
		manager.workers[instance.ID] = &worker{instance: instance}
	}
	go manager.watchLeases()
	return manager, nil
}

func (m *Manager) Restore() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, runtime := range m.workers {
		if runtime.instance.Mode == model.ModeInstalled && runtime.instance.DesiredState == model.DesiredRunning {
			if err := m.launchLocked(runtime); err != nil {
				m.failLocked(runtime, "worker_start_failed", err.Error(), "Run `spare doctor` and check the Site logs.")
			}
		}
	}
}

func (m *Manager) Create(request CreateRequest) (model.Instance, error) {
	root, err := site.ValidateRoot(request.RootPath)
	if err != nil {
		return model.Instance{}, &ManagerError{
			Code:    "invalid_site_folder",
			Message: err.Error(),
			Hint:    "Choose a readable folder that contains the files you want to share.",
		}
	}
	if request.Mode != model.ModeTemporary && request.Mode != model.ModeInstalled {
		return model.Instance{}, &ManagerError{Code: "invalid_mode", Message: "The Site mode is invalid."}
	}
	if request.PortMode == "" {
		request.PortMode = "auto"
	}

	m.mu.Lock()
	if existing, ok := m.workers[model.RecipeSite]; ok {
		if sameConfiguration(existing.instance, request, root) {
			instance := m.decorate(existing.instance)
			m.mu.Unlock()
			return instance, nil
		}
		m.mu.Unlock()
		return model.Instance{}, &ManagerError{
			Code:    "role_already_exists",
			Message: "This computer already has a Site.",
			Hint:    "Remove the current Site before choosing a different folder or port.",
		}
	}
	m.mu.Unlock()

	port, err := selectPort(request.Port, request.PortMode)
	if err != nil {
		return model.Instance{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return model.Instance{}, &ManagerError{Code: "daemon_stopping", Message: "Spare is stopping.", Hint: "Try again in a moment."}
	}
	if existing, ok := m.workers[model.RecipeSite]; ok {
		if sameConfiguration(existing.instance, request, root) {
			return m.decorate(existing.instance), nil
		}
		return model.Instance{}, &ManagerError{
			Code:    "role_already_exists",
			Message: "This computer already has a Site.",
			Hint:    "Remove the current Site before choosing a different folder or port.",
		}
	}

	now := time.Now().UTC()
	instance := model.Instance{
		ID:           model.RecipeSite,
		RecipeID:     model.RecipeSite,
		Mode:         request.Mode,
		DesiredState: model.DesiredRunning,
		Status:       model.StatusStarting,
		RootPath:     root,
		Port:         port,
		PortMode:     request.PortMode,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	runtime := &worker{instance: instance}
	if request.Mode == model.ModeTemporary {
		runtime.leaseUntil = now.Add(leaseDuration)
	}
	m.workers[instance.ID] = runtime
	if request.Mode == model.ModeInstalled {
		if err := m.store.PutInstance(context.Background(), instance); err != nil {
			delete(m.workers, instance.ID)
			return model.Instance{}, err
		}
	}
	if err := m.launchLocked(runtime); err != nil {
		delete(m.workers, instance.ID)
		if request.Mode == model.ModeInstalled {
			_ = m.store.DeleteInstance(context.Background(), instance.ID)
		}
		return model.Instance{}, &ManagerError{
			Code:    "worker_start_failed",
			Message: "Unable to start Site.",
			Hint:    err.Error(),
		}
	}
	m.eventLocked(runtime, "info", "site_created", "Site started.", map[string]any{"mode": request.Mode, "port": port})
	return m.decorate(runtime.instance), nil
}

func sameConfiguration(instance model.Instance, request CreateRequest, root string) bool {
	return instance.Mode == model.ModeInstalled &&
		request.Mode == model.ModeInstalled &&
		instance.RootPath == root &&
		instance.PortMode == request.PortMode &&
		(request.PortMode == "auto" || instance.Port == request.Port)
}

func (m *Manager) List() []model.Instance {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]model.Instance, 0, len(m.workers))
	for _, runtime := range m.workers {
		result = append(result, m.decorate(runtime.instance))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (m *Manager) Get(id string) (model.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.workers[id]
	if !ok {
		return model.Instance{}, &ManagerError{Code: "instance_not_found", Message: "Site is not installed.", Hint: "Run `spare install site --path <folder>` first."}
	}
	return m.decorate(runtime.instance), nil
}

func (m *Manager) Start(id string) (model.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.workers[id]
	if !ok {
		return model.Instance{}, &ManagerError{Code: "instance_not_found", Message: "Site is not installed."}
	}
	if runtime.cmd != nil && runtime.instance.DesiredState == model.DesiredRunning {
		return m.decorate(runtime.instance), nil
	}
	runtime.instance.DesiredState = model.DesiredRunning
	runtime.explicitStop = false
	runtime.crashes = nil
	if runtime.instance.Mode == model.ModeTemporary {
		runtime.leaseUntil = time.Now().Add(leaseDuration)
	}
	if err := m.launchLocked(runtime); err != nil {
		m.failLocked(runtime, "worker_start_failed", "Unable to start Site.", err.Error())
		return model.Instance{}, err
	}
	m.eventLocked(runtime, "info", "site_started", "Site started.", nil)
	return m.decorate(runtime.instance), nil
}

func (m *Manager) Stop(id string) (model.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.workers[id]
	if !ok {
		return model.Instance{}, &ManagerError{Code: "instance_not_found", Message: "Site is not installed."}
	}
	if runtime.instance.DesiredState == model.DesiredStopped && runtime.cmd == nil {
		return m.decorate(runtime.instance), nil
	}
	runtime.instance.DesiredState = model.DesiredStopped
	runtime.explicitStop = true
	m.stopProcessLocked(runtime)
	runtime.instance.Status = model.StatusStopped
	runtime.instance.Problem = nil
	runtime.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(runtime)
	m.eventLocked(runtime, "info", "site_stopped", "Site stopped.", nil)
	return m.decorate(runtime.instance), nil
}

func (m *Manager) Heartbeat(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.workers[id]
	if !ok || runtime.instance.Mode != model.ModeTemporary {
		return &ManagerError{Code: "temporary_instance_not_found", Message: "The temporary Site is no longer running."}
	}
	runtime.leaseUntil = time.Now().Add(leaseDuration)
	return nil
}

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.workers[id]
	if !ok {
		return &ManagerError{Code: "instance_not_found", Message: "Site is not installed."}
	}
	runtime.instance.Status = model.StatusRemoving
	runtime.instance.DesiredState = model.DesiredStopped
	runtime.explicitStop = true
	m.stopProcessLocked(runtime)
	if runtime.instance.Mode == model.ModeInstalled {
		if err := m.store.DeleteInstance(context.Background(), id); err != nil && !state.IsNotFound(err) {
			return err
		}
	}
	delete(m.workers, id)
	_ = os.Remove(filepath.Join(m.logsDir, id+".log"))
	for index := 1; index <= 5; index++ {
		_ = os.Remove(fmt.Sprintf("%s.%d", filepath.Join(m.logsDir, id+".log"), index))
	}
	_ = m.store.AddEvent(context.Background(), model.Event{
		InstanceID: id,
		Level:      "info",
		Kind:       "site_removed",
		Message:    "Site was removed. The served folder was left unchanged.",
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
	for _, runtime := range m.workers {
		if runtime.restartTimer != nil {
			runtime.restartTimer.Stop()
		}
		m.stopProcessLocked(runtime)
	}
	m.mu.Unlock()
	select {
	case <-m.waitDone:
	case <-time.After(2 * time.Second):
	}
}

func (m *Manager) launchLocked(runtime *worker) error {
	if runtime.cmd != nil {
		return nil
	}
	port := runtime.instance.Port
	if !portAvailable(port) {
		if runtime.instance.PortMode == "auto" {
			selected, err := selectPort(0, "auto")
			if err != nil {
				return err
			}
			port = selected
			runtime.instance.Port = selected
			m.eventLocked(runtime, "warning", "port_changed", "Site moved to a free local port.", map[string]any{"port": selected})
		} else {
			return &ManagerError{
				Code:    "port_in_use",
				Message: fmt.Sprintf("Port %d is already in use.", port),
				Hint:    "Remove and reinstall Site with `--port auto` or another port.",
			}
		}
	}
	healthPort, err := freeLoopbackPort()
	if err != nil {
		return err
	}
	logWriter, err := logs.NewRotatingWriter(filepath.Join(m.logsDir, runtime.instance.ID+".log"), 5*1024*1024, 5)
	if err != nil {
		return err
	}
	command := exec.CommandContext(
		m.ctx,
		m.exe,
		"worker",
		"site",
		"--path",
		runtime.instance.RootPath,
		"--port",
		fmt.Sprintf("%d", runtime.instance.Port),
		"--health-port",
		fmt.Sprintf("%d", healthPort),
	)
	command.Stdout = logWriter
	command.Stderr = logWriter
	if err := command.Start(); err != nil {
		_ = logWriter.Close()
		return err
	}

	runtime.generation++
	generation := runtime.generation
	runtime.cmd = command
	runtime.log = logWriter
	runtime.healthPort = healthPort
	runtime.healthFails = 0
	runtime.healthyBefore = false
	runtime.instance.Status = model.StatusStarting
	runtime.instance.Problem = nil
	runtime.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(runtime)

	go m.wait(runtime.instance.ID, generation, command)
	go m.monitor(runtime.instance.ID, generation, healthPort)
	return nil
}

func (m *Manager) wait(id string, generation uint64, command *exec.Cmd) {
	err := command.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.workers[id]
	if !ok || runtime.generation != generation || runtime.cmd != command {
		return
	}
	if runtime.log != nil {
		_ = runtime.log.Close()
		runtime.log = nil
	}
	runtime.cmd = nil
	m.stopMDNSLocked(runtime)

	if runtime.instance.DesiredState == model.DesiredStopped || runtime.explicitStop || m.closed {
		runtime.instance.Status = model.StatusStopped
		runtime.instance.UpdatedAt = time.Now().UTC()
		m.persistLocked(runtime)
		return
	}
	if runtime.instance.Mode == model.ModeTemporary && time.Now().After(runtime.leaseUntil) {
		delete(m.workers, id)
		return
	}

	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)
	var crashes []time.Time
	for _, crash := range runtime.crashes {
		if crash.After(cutoff) {
			crashes = append(crashes, crash)
		}
	}
	runtime.crashes = append(crashes, now)
	if len(runtime.crashes) >= 5 {
		m.failLocked(runtime, "restart_limit_reached", "Site stopped after repeatedly failing.", "Check `spare logs site`, then run `spare start site` after fixing the problem.")
		return
	}

	delay := time.Second << (len(runtime.crashes) - 1)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	message := "Site stopped unexpectedly. Spare will restart it."
	if err != nil {
		message = fmt.Sprintf("Site stopped unexpectedly (%s). Spare will restart it.", err)
	}
	runtime.instance.Status = model.StatusDegraded
	runtime.instance.Problem = &model.Problem{
		Code:     "worker_exited",
		Severity: "warning",
		Summary:  message,
		Recovery: "Spare is restarting Site automatically.",
	}
	runtime.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(runtime)
	m.eventLocked(runtime, "warning", "worker_exited", message, map[string]any{"restartInSeconds": int(delay.Seconds())})
	runtime.restartTimer = time.AfterFunc(delay, func() {
		m.restart(id, generation)
	})
}

func (m *Manager) restart(id string, generation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.workers[id]
	if !ok || runtime.generation != generation || runtime.cmd != nil || runtime.instance.DesiredState != model.DesiredRunning || m.closed {
		return
	}
	if runtime.instance.Mode == model.ModeTemporary && time.Now().After(runtime.leaseUntil) {
		delete(m.workers, id)
		return
	}
	if err := m.launchLocked(runtime); err != nil {
		m.failLocked(runtime, "worker_restart_failed", "Unable to restart Site.", err.Error())
	}
}

func (m *Manager) monitor(id string, generation uint64, healthPort int) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	checks := 0
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-timer.C:
			checks++
			if checks > 1 {
				timer.Reset(healthEvery)
			}
			response, err := m.http.Get(fmt.Sprintf("http://127.0.0.1:%d/", healthPort))
			healthy := err == nil && response.StatusCode == http.StatusOK
			if response != nil {
				_ = response.Body.Close()
			}
			m.mu.Lock()
			runtime, ok := m.workers[id]
			if !ok || runtime.generation != generation || runtime.cmd == nil {
				m.mu.Unlock()
				return
			}
			if healthy {
				runtime.healthFails = 0
				if !runtime.healthyBefore {
					now := time.Now().UTC()
					runtime.healthyBefore = true
					runtime.instance.Status = model.StatusHealthy
					runtime.instance.Problem = nil
					runtime.instance.StartedAt = &now
					runtime.instance.UpdatedAt = now
					m.persistLocked(runtime)
					if runtime.mdns == nil {
						runtime.mdns, _ = discovery.Advertise(m.machine.Hostname, runtime.instance.Port)
					}
					m.eventLocked(runtime, "info", "site_healthy", "Site is ready.", nil)
				}
				m.mu.Unlock()
				continue
			}
			runtime.healthFails++
			if runtime.healthFails >= 3 {
				runtime.instance.Status = model.StatusDegraded
				runtime.instance.Problem = &model.Problem{
					Code:     "health_check_failed",
					Severity: "warning",
					Summary:  "Site stopped responding.",
					Recovery: "Spare is restarting Site automatically.",
				}
				runtime.instance.UpdatedAt = time.Now().UTC()
				m.persistLocked(runtime)
				if runtime.cmd != nil && runtime.cmd.Process != nil {
					_ = runtime.cmd.Process.Kill()
				}
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
			m.mu.Lock()
			now := time.Now()
			for id, runtime := range m.workers {
				if runtime.instance.Mode != model.ModeTemporary || now.Before(runtime.leaseUntil) {
					continue
				}
				runtime.instance.DesiredState = model.DesiredStopped
				runtime.explicitStop = true
				m.stopProcessLocked(runtime)
				delete(m.workers, id)
				_ = m.store.AddEvent(context.Background(), model.Event{
					InstanceID: id,
					Level:      "info",
					Kind:       "temporary_site_expired",
					Message:    "Temporary Site stopped after its terminal closed.",
				})
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) stopProcessLocked(runtime *worker) {
	if runtime.restartTimer != nil {
		runtime.restartTimer.Stop()
		runtime.restartTimer = nil
	}
	m.stopMDNSLocked(runtime)
	if runtime.cmd != nil && runtime.cmd.Process != nil {
		runtime.generation++
		command := runtime.cmd
		runtime.cmd = nil
		_ = command.Process.Kill()
	}
	if runtime.log != nil {
		_ = runtime.log.Close()
		runtime.log = nil
	}
}

func (m *Manager) stopMDNSLocked(runtime *worker) {
	if runtime.mdns != nil {
		_ = runtime.mdns.Close()
		runtime.mdns = nil
	}
}

func (m *Manager) failLocked(runtime *worker, code, summary, recovery string) {
	runtime.instance.Status = model.StatusFailed
	runtime.instance.Problem = &model.Problem{
		Code:     code,
		Severity: "error",
		Summary:  summary,
		Recovery: recovery,
	}
	runtime.instance.UpdatedAt = time.Now().UTC()
	m.persistLocked(runtime)
	m.eventLocked(runtime, "error", code, summary, nil)
}

func (m *Manager) persistLocked(runtime *worker) {
	if runtime.instance.Mode == model.ModeInstalled {
		_ = m.store.PutInstance(context.Background(), runtime.instance)
	}
}

func (m *Manager) eventLocked(runtime *worker, level, kind, message string, details map[string]any) {
	_ = m.store.AddEvent(context.Background(), model.Event{
		InstanceID: runtime.instance.ID,
		Level:      level,
		Kind:       kind,
		Message:    message,
		Details:    details,
	})
}

func (m *Manager) decorate(instance model.Instance) model.Instance {
	urls := []string{fmt.Sprintf("http://127.0.0.1:%d", instance.Port)}
	addresses := profile.LANAddresses()
	for _, address := range addresses {
		urls = append(urls, "http://"+net.JoinHostPort(address, fmt.Sprintf("%d", instance.Port)))
	}
	if hostname := mdnsHostname(m.machine.Hostname); hostname != "" {
		urls = append(urls, fmt.Sprintf("http://%s.local:%d", hostname, instance.Port))
	}
	instance.URLs = urls
	return instance
}

func selectPort(requested int, mode string) (int, error) {
	if mode == "fixed" || requested > 0 {
		if requested < 1 || requested > 65535 {
			return 0, &ManagerError{Code: "invalid_port", Message: "Choose a port between 1 and 65535."}
		}
		if !portAvailable(requested) {
			return 0, &ManagerError{
				Code:    "port_in_use",
				Message: fmt.Sprintf("Port %d is already in use.", requested),
				Hint:    "Use `--port auto` or choose another port.",
			}
		}
		return requested, nil
	}
	for port := 7340; port <= 7399; port++ {
		if portAvailable(port) {
			return port, nil
		}
	}
	return 0, &ManagerError{
		Code:    "no_site_port_available",
		Message: "Spare could not find a free Site port.",
		Hint:    "Close another local service or choose a specific free port.",
	}
}

func portAvailable(port int) bool {
	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func freeLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func mdnsHostname(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".local")
	var result strings.Builder
	lastDash := false
	for _, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if valid {
			result.WriteRune(character)
			lastDash = false
		} else if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}
