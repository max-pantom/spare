package runtime

import (
	"context"
	"io"

	"github.com/spare-run/spare/internal/model"
)

type Status struct {
	Running bool
	PID     int
}

type Process interface {
	Wait() error
	Stop() error
	Status() Status
}

type Runtime interface {
	Name() string
	Prepare(ctx context.Context, instance model.Instance) error
	Start(ctx context.Context, instance model.Instance, healthPort int, stdout, stderr io.Writer) (Process, error)
	Stop(ctx context.Context, instance model.Instance, process Process) error
	Status(ctx context.Context, instance model.Instance, process Process) (Status, error)
	Remove(ctx context.Context, instance model.Instance) error
}
