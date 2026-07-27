package process

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"

	"github.com/spare-run/spare/internal/model"
	spareRuntime "github.com/spare-run/spare/internal/runtime"
)

type Driver struct {
	Executable string
	Arguments  []string
}

func (d *Driver) Name() string {
	return "process"
}

func (d *Driver) Prepare(context.Context, model.Instance) error {
	if d.Executable == "" {
		return errors.New("approved process executable is not configured")
	}
	return nil
}

func (d *Driver) Start(ctx context.Context, instance model.Instance, _ int, stdout, stderr io.Writer) (spareRuntime.Process, error) {
	if err := d.Prepare(ctx, instance); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, d.Executable, d.Arguments...)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	return &running{command: command}, nil
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

type running struct {
	mu      sync.Mutex
	command *exec.Cmd
	waited  bool
}

func (p *running) Wait() error {
	err := p.command.Wait()
	p.mu.Lock()
	p.waited = true
	p.mu.Unlock()
	return err
}

func (p *running) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waited || p.command.Process == nil {
		return nil
	}
	return p.command.Process.Kill()
}

func (p *running) Status() spareRuntime.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waited || p.command.Process == nil {
		return spareRuntime.Status{}
	}
	return spareRuntime.Status{Running: true, PID: p.command.Process.Pid}
}
