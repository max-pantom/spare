package native

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"

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
	command := exec.CommandContext(
		ctx,
		d.Executable,
		"worker",
		"--recipe", instance.RecipeID,
		"--config", base64.RawURLEncoding.EncodeToString(configJSON),
		"--port", strconv.Itoa(instance.Port),
		"--health-port", strconv.Itoa(healthPort),
	)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	return &process{command: command}, nil
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
	if err := p.command.Process.Kill(); err != nil && !errors.Is(err, exec.ErrNotFound) {
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
