//go:build linux

package isolation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/spare-run/spare/internal/model"
	"golang.org/x/sys/unix"
)

const policyEnvironment = "SPARE_WORKER_ISOLATION"

func Command(ctx context.Context, executable string, workerArgs []string, policy model.WorkerIsolation) (*exec.Cmd, func(), error) {
	data, err := json.Marshal(policy)
	if err != nil {
		return nil, nil, err
	}
	args := append([]string{"sandbox-worker"}, workerArgs...)
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = append(cleanEnvironment(), policyEnvironment+"="+base64.RawURLEncoding.EncodeToString(data))
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	return command, func() {}, nil
}

func Enter(workerArgs []string) error {
	encoded := os.Getenv(policyEnvironment)
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return errors.New("the worker isolation policy is invalid")
	}
	var policy model.WorkerIsolation
	if err := json.Unmarshal(data, &policy); err != nil {
		return errors.New("the worker isolation policy is invalid")
	}
	if err := applyLandlock(policy); err != nil {
		return fmt.Errorf("apply Linux worker filesystem isolation: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	args := append([]string{executable, "worker"}, workerArgs...)
	return unix.Exec(executable, args, cleanEnvironment())
}

func applyLandlock(policy model.WorkerIsolation) error {
	abi, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		0,
		0,
		unix.LANDLOCK_CREATE_RULESET_VERSION,
	)
	if errno != 0 {
		return fmt.Errorf("Landlock is unavailable: %w", errno)
	}
	handled := uint64(
		unix.LANDLOCK_ACCESS_FS_EXECUTE |
			unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
			unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
			unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
			unix.LANDLOCK_ACCESS_FS_MAKE_REG |
			unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
			unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_SYM,
	)
	if abi >= 2 {
		handled |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		handled |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	if abi >= 5 {
		handled |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
	}
	attribute := unix.LandlockRulesetAttr{Access_fs: handled}
	ruleset, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attribute)),
		unsafe.Sizeof(attribute),
		0,
	)
	runtime.KeepAlive(attribute)
	if errno != 0 {
		return errno
	}
	defer unix.Close(int(ruleset))

	read := uint64(unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR)
	systemRead := read | unix.LANDLOCK_ACCESS_FS_EXECUTE
	write := handled &^ uint64(unix.LANDLOCK_ACCESS_FS_EXECUTE)
	for _, path := range []string{"/bin", "/sbin", "/usr", "/lib", "/lib64", "/etc", "/dev", "/proc", "/sys", "/run"} {
		if err := addLandlockPath(int(ruleset), path, systemRead, true); err != nil {
			return err
		}
	}
	if err := addLandlockPath(int(ruleset), os.TempDir(), write, false); err != nil {
		return err
	}
	if err := addLandlockPath(int(ruleset), policy.StatePath, write, false); err != nil {
		return err
	}
	for _, path := range policy.ReadPaths {
		if err := addLandlockPath(int(ruleset), path, read, false); err != nil {
			return err
		}
	}
	for _, path := range policy.WritePaths {
		if err := addLandlockPath(int(ruleset), path, write, false); err != nil {
			return err
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	_, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, ruleset, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func addLandlockPath(ruleset int, path string, access uint64, optional bool) error {
	if path == "" {
		return errors.New("worker isolation path is empty")
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && optional {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	attribute := unix.LandlockPathBeneathAttr{Allowed_access: access, Parent_fd: int32(file.Fd())}
	_, _, errno := unix.Syscall6(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(ruleset),
		unix.LANDLOCK_RULE_PATH_BENEATH,
		uintptr(unsafe.Pointer(&attribute)),
		0, 0, 0,
	)
	runtime.KeepAlive(attribute)
	if errno != 0 {
		return fmt.Errorf("allow %s: %w", path, errno)
	}
	return nil
}

func ContainProcess(*os.Process) (func(), error) {
	return func() {}, nil
}

func Terminate(process *os.Process) error {
	if process == nil {
		return nil
	}
	if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return process.Kill()
	}
	return nil
}
