package native

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/spare-run/spare/internal/isolation"
	"github.com/spare-run/spare/internal/model"
	spareRuntime "github.com/spare-run/spare/internal/runtime"
)

type Driver struct {
	Executable string
}

func (d *Driver) Name() string {
	return "native"
}

func (d *Driver) Prepare(context.Context, model.Instance) error {
	if d.Executable == "" {
		return errors.New("native runtime executable is not configured")
	}
	return nil
}

func (d *Driver) Start(ctx context.Context, instance model.Instance, healthPort int, stdout, stderr io.Writer) (spareRuntime.Process, error) {
	if err := d.Prepare(ctx, instance); err != nil {
		return nil, err
	}
	configJSON, err := json.Marshal(instance.Config)
	if err != nil {
		return nil, err
	}
	workerArgs := []string{
		"--recipe", instance.RecipeID,
		"--config-stdin",
		"--port", strconv.Itoa(instance.Port),
		"--health-port", strconv.Itoa(healthPort),
		"--data-path", instance.StatePath,
	}
	command, sandboxCleanup, err := isolation.Command(ctx, d.Executable, workerArgs, instance.Isolation)
	if err != nil {
		return nil, fmt.Errorf("prepare native worker isolation: %w", err)
	}
	command.Stdin = bytes.NewReader(configJSON)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		sandboxCleanup()
		return nil, err
	}
	containCleanup, err := isolation.ContainProcess(command.Process)
	if err != nil {
		_ = isolation.Terminate(command.Process)
		_ = command.Wait()
		sandboxCleanup()
		return nil, fmt.Errorf("contain native worker process: %w", err)
	}
	return &process{
		command: command,
		cleanup: func() {
			containCleanup()
			sandboxCleanup()
		},
	}, nil
}

func (d *Driver) Stop(_ context.Context, _ model.Instance, process spareRuntime.Process) error {
	if process == nil {
		return nil
	}
	return process.Stop()
}

func (d *Driver) Status(_ context.Context, _ model.Instance, process spareRuntime.Process) (spareRuntime.Status, error) {
	if process == nil {
		return spareRuntime.Status{}, nil
	}
	return process.Status(), nil
}

func (d *Driver) Remove(context.Context, model.Instance) error {
	return nil
}

type process struct {
	mu      sync.Mutex
	command *exec.Cmd
	waited  bool
	waitErr error
	cleanup func()
	cleaned sync.Once
}

func (p *process) Wait() error {
	p.mu.Lock()
	if p.waited {
		err := p.waitErr
		p.mu.Unlock()
		return err
	}
	command := p.command
	p.mu.Unlock()
	err := command.Wait()
	p.cleaned.Do(p.cleanup)
	p.mu.Lock()
	p.waited = true
	p.waitErr = err
	p.mu.Unlock()
	return err
}

func (p *process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waited || p.command == nil || p.command.Process == nil {
		return nil
	}
	p.cleaned.Do(p.cleanup)
	if err := isolation.Terminate(p.command.Process); err != nil && !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop native worker: %w", err)
	}
	return nil
}

func (p *process) Status() spareRuntime.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.command == nil || p.command.Process == nil || p.waited {
		return spareRuntime.Status{}
	}
	return spareRuntime.Status{Running: true, PID: p.command.Process.Pid}
}
