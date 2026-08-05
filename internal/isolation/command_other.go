//go:build !darwin && !linux && !windows

package isolation

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/spare-run/spare/internal/model"
)

func Command(ctx context.Context, executable string, workerArgs []string, _ model.WorkerIsolation) (*exec.Cmd, func(), error) {
	args := append([]string{"worker"}, workerArgs...)
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = cleanEnvironment()
	return command, func() {}, nil
}

func Enter([]string) error                       { return errors.New("worker isolation is unsupported") }
func ContainProcess(*os.Process) (func(), error) { return func() {}, nil }
func Terminate(process *os.Process) error        { return process.Kill() }
