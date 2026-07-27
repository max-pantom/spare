package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/auth"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/profile"
	"github.com/spare-run/spare/internal/service"
	"github.com/spare-run/spare/internal/state"
)

// Ensure prepares user state, registers the daemon, starts it, and waits for
// its authenticated loopback API. The CLI and desktop app share this path.
func Ensure(ctx context.Context, statePaths paths.Paths, daemonPath string) (*api.Client, model.Machine, error) {
	if err := statePaths.Ensure(); err != nil {
		return nil, model.Machine{}, err
	}
	if _, err := auth.EnsureToken(statePaths.Token); err != nil {
		return nil, model.Machine{}, err
	}
	if err := profileState(ctx, statePaths); err != nil {
		return nil, model.Machine{}, err
	}
	if daemonPath == "" {
		var err error
		daemonPath, err = FindDaemon()
		if err != nil {
			return nil, model.Machine{}, err
		}
	}
	if err := service.InstallAndStart(ctx, daemonPath, statePaths.Root); err != nil {
		return nil, model.Machine{}, err
	}
	client, err := WaitForDaemon(ctx, statePaths, 10*time.Second)
	if err != nil {
		return nil, model.Machine{}, err
	}
	machine, err := client.Machine(ctx)
	if err != nil {
		return nil, model.Machine{}, err
	}
	return client, machine, nil
}

// Recover connects to a healthy daemon when possible and otherwise performs
// the same idempotent initialization as first launch.
func Recover(ctx context.Context, statePaths paths.Paths, daemonPath string) (*api.Client, model.Machine, error) {
	if client, err := api.Discover(statePaths); err == nil {
		checkContext, cancel := context.WithTimeout(ctx, time.Second)
		healthErr := client.Health(checkContext)
		cancel()
		if healthErr == nil {
			machine, machineErr := client.Machine(ctx)
			return client, machine, machineErr
		}
	}
	return Ensure(ctx, statePaths, daemonPath)
}

func WaitForDaemon(ctx context.Context, statePaths paths.Paths, timeout time.Duration) (*api.Client, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		client, err := api.Discover(statePaths)
		if err == nil {
			checkContext, cancel := context.WithTimeout(ctx, time.Second)
			err = client.Health(checkContext)
			cancel()
			if err == nil {
				return client, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return nil, errors.New("Spare did not start. Run `spare doctor` and check the daemon log")
}

func FindDaemon() (string, error) {
	if configured := os.Getenv("SPARED_PATH"); configured != "" {
		return filepath.Abs(configured)
	}
	current, err := os.Executable()
	if err == nil {
		name := "spared"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		sibling := filepath.Join(filepath.Dir(current), name)
		if _, statErr := os.Stat(sibling); statErr == nil {
			return sibling, nil
		}
	}
	name := "spared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", errors.New("could not find `spared`; install the Spare desktop app, CLI, and daemon together")
	}
	return filepath.Abs(path)
}

func profileState(ctx context.Context, statePaths paths.Paths) error {
	store, err := state.Open(statePaths.Database)
	if err != nil {
		return err
	}
	defer store.Close()
	var existing *model.Machine
	if current, readErr := store.Machine(ctx); readErr == nil {
		existing = &current
	}
	machine, err := profile.Collect(existing, statePaths.Root)
	if err != nil {
		return fmt.Errorf("profile this computer: %w", err)
	}
	return store.SaveMachine(ctx, machine)
}
