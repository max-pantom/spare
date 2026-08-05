//go:build desktop

package desktop

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/bootstrap"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/recipes/shared/pairing"
	"github.com/spare-run/spare/internal/uninstall"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	paths         paths.Paths
	daemonPath    string
	startHidden   bool
	launchPaths   []string
	frontendReady bool

	ctx       context.Context
	client    *api.Client
	connectMu sync.Mutex
	stateMu   sync.RWMutex
	last      Snapshot
	quitting  bool
	tray      trayController
}

func New(statePaths paths.Paths, daemonPath string, startHidden bool, launchPaths []string) *App {
	return &App{
		paths:       statePaths,
		daemonPath:  daemonPath,
		startHidden: startHidden,
		launchPaths: append([]string(nil), launchPaths...),
		tray:        newTrayController(),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	preferences := loadPreferences(a.paths.Root)
	if os.Getenv("SPARE_DISABLE_TRAY") != "1" {
		a.tray.Start(a)
		a.tray.SetVisible(preferences.ShowInMenuBar)
	}
	if os.Getenv("SPARE_NO_SERVICE") != "1" {
		_ = configureDesktopLogin(preferences)
	}
	wailsruntime.OnFileDrop(ctx, func(_ int, _ int, dropped []string) {
		wailsruntime.EventsEmit(ctx, "spare:file-drop", dropped)
	})
	go func() {
		if _, err := a.ensureConnected(ctx); err == nil {
			a.startBackgroundLoops()
		}
	}()
}

func (a *App) DomReady(ctx context.Context) {
	if !a.startHidden {
		a.showWindow()
	}
}

func (a *App) Shutdown(context.Context) {
	a.tray.Stop()
}

// BeforeClose only prompts during an intentional quit. Closing the window
// hides it and keeps the menu-bar process and temporary lease alive.
func (a *App) BeforeClose(ctx context.Context) bool {
	a.stateMu.RLock()
	quitting := a.quitting
	a.stateMu.RUnlock()
	if !quitting {
		return false
	}
	return a.confirmTemporaryQuit(ctx)
}

func (a *App) Bootstrap() (Snapshot, error) {
	if a.ctx == nil {
		return Snapshot{}, errors.New("Spare desktop is still starting")
	}
	if _, err := a.ensureConnected(a.ctx); err != nil {
		return Snapshot{}, err
	}
	return a.Snapshot()
}

func (a *App) Snapshot() (Snapshot, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return Snapshot{}, err
	}
	machine, err := client.Machine(a.ctx)
	if err != nil {
		return Snapshot{}, err
	}
	recipes, err := client.Recipes(a.ctx)
	if err != nil {
		return Snapshot{}, err
	}
	instances, err := client.Instances(a.ctx)
	if err != nil {
		return Snapshot{}, err
	}
	events, err := client.Events(a.ctx, 50)
	if err != nil {
		return Snapshot{}, err
	}
	devices := []pairing.ConnectedDevice{}
	if len(instances) > 0 {
		devices, _ = pairing.ReadConnectedDevices(
			filepath.Join(a.paths.JobData, instances[0].RecipeID),
			time.Now(),
		)
	}
	snapshot := Snapshot{
		Surface:     "desktop",
		Machine:     machine,
		Recipes:     recipes,
		Instances:   instances,
		Events:      events,
		Devices:     devices,
		Preferences: loadPreferences(a.paths.Root),
	}
	a.stateMu.Lock()
	a.last = snapshot
	a.stateMu.Unlock()
	a.tray.Update(snapshot)
	return snapshot, nil
}

func (a *App) CreateInstance(input CreateInput) (model.Instance, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.Instance{}, err
	}
	instance, err := client.Create(a.ctx, input.RecipeID, input.Mode, input.Config, input.PortMode, input.Port)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return instance, err
}

func (a *App) SwitchInstance(input CreateInput) (model.Instance, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.Instance{}, err
	}
	instance, err := client.Switch(a.ctx, input.RecipeID, input.Mode, input.Config, input.PortMode, input.Port)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return instance, err
}

func (a *App) ReviewJobPackage(source string) (model.JobPackageReview, error) {
	if source == "" {
		selected, err := a.ChooseFile("recipe")
		if err != nil || selected == "" {
			return model.JobPackageReview{}, err
		}
		source = selected
	}
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.JobPackageReview{}, err
	}
	return client.ReviewJobPackage(a.ctx, source)
}

func (a *App) InstallJobPackage(source string) (model.JobPackage, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.JobPackage{}, err
	}
	value, err := client.InstallJobPackage(a.ctx, source)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return value, err
}

func (a *App) UninstallJobPackage(id string) error {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return err
	}
	if err := client.UninstallJobPackage(a.ctx, id); err != nil {
		return err
	}
	_, _ = a.Snapshot()
	return nil
}

func (a *App) JobProfile(id string) (model.JobProfile, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.JobProfile{}, err
	}
	return client.JobProfile(a.ctx, id)
}

func (a *App) OpenJobCatalog() error {
	value := os.Getenv("SPARE_CATALOG_URL")
	if value == "" {
		value = "https://spare.run/jobs/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return errors.New("the Spare job catalog address is invalid")
	}
	loopback := false
	if address, parseErr := netip.ParseAddr(parsed.Hostname()); parseErr == nil {
		loopback = address.IsLoopback()
	}
	if parsed.Scheme != "https" &&
		!(parsed.Scheme == "http" &&
			(loopback || parsed.Hostname() == "localhost")) {
		return errors.New("Spare opens job catalogs only over HTTPS")
	}
	wailsruntime.BrowserOpenURL(a.ctx, value)
	return nil
}

func (a *App) StartInstance(id string) (model.Instance, error) {
	return a.instanceAction(id, "start")
}

func (a *App) StopInstance(id string) (model.Instance, error) {
	return a.instanceAction(id, "stop")
}

func (a *App) PromoteInstance(id string) (model.Instance, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.Instance{}, err
	}
	instance, err := client.Promote(a.ctx, id)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return instance, err
}

func (a *App) ConfigureInstance(id string, input CreateInput) (model.Instance, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.Instance{}, err
	}
	instance, err := client.Configure(
		a.ctx,
		id,
		input.RecipeID,
		input.Config,
		input.PortMode,
		input.Port,
	)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return instance, err
}

func (a *App) RemoveInstance(id string) error {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return err
	}
	if err := client.Remove(a.ctx, id); err != nil {
		return err
	}
	_, _ = a.Snapshot()
	return nil
}

func (a *App) Repair() (Snapshot, error) {
	a.connectMu.Lock()
	a.client = nil
	client, _, err := bootstrap.Ensure(a.ctx, a.paths, a.daemonPath)
	if err == nil {
		a.client = client
	}
	a.connectMu.Unlock()
	if err != nil {
		return Snapshot{}, err
	}
	return a.Snapshot()
}

func (a *App) Preferences() Preferences {
	return loadPreferences(a.paths.Root)
}

func (a *App) SavePreferences(preferences Preferences) error {
	if err := configureDesktopLogin(preferences); err != nil {
		return err
	}
	if err := savePreferences(a.paths.Root, preferences); err != nil {
		return err
	}
	if preferences.Notifications {
		_ = wailsruntime.InitializeNotifications(a.ctx)
		_, _ = wailsruntime.RequestNotificationAuthorization(a.ctx)
	}
	a.tray.SetVisible(preferences.ShowInMenuBar)
	wailsruntime.EventsEmit(a.ctx, "spare:preferences", preferences)
	return nil
}

func (a *App) ShowWindow() {
	a.showWindow()
}

func (a *App) PendingLaunchPaths() []string {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	result := append([]string(nil), a.launchPaths...)
	a.launchPaths = nil
	a.frontendReady = true
	return result
}

func (a *App) QueueLaunchPaths(paths []string) {
	if len(paths) == 0 {
		return
	}
	a.stateMu.Lock()
	ctx := a.ctx
	ready := a.frontendReady
	if !ready {
		a.launchPaths = append(a.launchPaths, paths...)
	}
	a.stateMu.Unlock()
	a.showWindow()
	if ctx != nil && ready {
		wailsruntime.EventsEmit(ctx, "spare:file-drop", paths)
	}
}

func (a *App) Quit() {
	a.requestQuit()
}

func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	// Show restores the application itself on macOS; WindowShow alone only
	// changes the webview window state and can leave a directly launched app
	// running without becoming visible.
	wailsruntime.Show(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
}

func (a *App) Uninstall() error {
	client, err := a.ensureConnected(a.ctx)
	if err == nil {
		instances, _ := client.Instances(a.ctx)
		for _, instance := range instances {
			_ = client.Remove(a.ctx, instance.ID)
		}
	}
	if err := configureDesktopLogin(Preferences{}); err != nil {
		return err
	}
	if err := uninstall.Remove(a.ctx, a.paths); err != nil {
		return err
	}
	if executable, executableErr := os.Executable(); executableErr == nil {
		if helper, command, arguments := desktopUninstallCommand(executable); helper != "" {
			if info, statErr := os.Stat(helper); statErr == nil && !info.IsDir() {
				_ = exec.Command(command, arguments...).Start()
			}
		}
	}
	a.stateMu.Lock()
	a.quitting = true
	a.stateMu.Unlock()
	go func() {
		time.Sleep(150 * time.Millisecond)
		wailsruntime.Quit(a.ctx)
	}()
	return nil
}

func desktopUninstallCommand(executable string) (string, string, []string) {
	executableDirectory := filepath.Dir(executable)
	switch runtime.GOOS {
	case "darwin":
		helper := filepath.Clean(filepath.Join(executableDirectory, "..", "Resources", "uninstall.sh"))
		return helper, "/bin/sh", []string{helper}
	case "windows":
		helper := filepath.Join(executableDirectory, "uninstall.ps1")
		return helper,
			"powershell.exe",
			[]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", helper, "-FromApp"}
	default:
		helper := filepath.Join(executableDirectory, "uninstall.sh")
		return helper, "/bin/sh", []string{helper, "--from-app"}
	}
}

func (a *App) instanceAction(id, action string) (model.Instance, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.Instance{}, err
	}
	instance, err := client.InstanceAction(a.ctx, id, action)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return instance, err
}

func (a *App) ensureConnected(ctx context.Context) (*api.Client, error) {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	if a.client != nil {
		checkContext, cancel := context.WithTimeout(ctx, time.Second)
		err := a.client.Health(checkContext)
		cancel()
		if err == nil {
			return a.client, nil
		}
		a.client = nil
	}
	client, _, err := bootstrap.Recover(ctx, a.paths, a.daemonPath)
	if err != nil {
		return nil, err
	}
	a.client = client
	return client, nil
}
