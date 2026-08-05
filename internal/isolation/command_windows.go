//go:build windows

package isolation

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"unsafe"

	"github.com/spare-run/spare/internal/model"
	"golang.org/x/sys/windows"
)

func Command(ctx context.Context, executable string, workerArgs []string, _ model.WorkerIsolation) (*exec.Cmd, func(), error) {
	args := append([]string{"worker"}, workerArgs...)
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = cleanEnvironment()
	return command, func() {}, nil
}

func Enter([]string) error {
	return errors.New("nested Windows worker sandbox entry is not supported")
}

func ContainProcess(process *os.Process) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	closeJob := sync.OnceFunc(func() { _ = windows.CloseHandle(job) })
	failed := true
	defer func() {
		if failed {
			closeJob()
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION |
		windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
	limits.BasicLimitInformation.ActiveProcessLimit = 8
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		return nil, err
	}
	ui := windows.JOBOBJECT_BASIC_UI_RESTRICTIONS{UIRestrictionsClass: windows.JOB_OBJECT_UILIMIT_DESKTOP |
		windows.JOB_OBJECT_UILIMIT_DISPLAYSETTINGS | windows.JOB_OBJECT_UILIMIT_EXITWINDOWS |
		windows.JOB_OBJECT_UILIMIT_GLOBALATOMS | windows.JOB_OBJECT_UILIMIT_HANDLES |
		windows.JOB_OBJECT_UILIMIT_READCLIPBOARD | windows.JOB_OBJECT_UILIMIT_SYSTEMPARAMETERS |
		windows.JOB_OBJECT_UILIMIT_WRITECLIPBOARD}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectBasicUIRestrictions,
		uintptr(unsafe.Pointer(&ui)),
		uint32(unsafe.Sizeof(ui)),
	); err != nil {
		return nil, err
	}
	processHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(process.Pid))
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(processHandle)
	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		return nil, err
	}
	failed = false
	return closeJob, nil
}

func Terminate(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Kill()
}
