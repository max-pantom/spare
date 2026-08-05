//go:build darwin

package isolation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spare-run/spare/internal/model"
)

func Command(ctx context.Context, executable string, workerArgs []string, policy model.WorkerIsolation) (*exec.Cmd, func(), error) {
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		return nil, nil, errors.New("macOS worker sandbox is unavailable")
	}
	profile, err := macOSProfile(executable, policy)
	if err != nil {
		return nil, nil, err
	}
	profileFile, err := os.CreateTemp(policy.StatePath, ".worker-sandbox-*.sb")
	if err != nil {
		return nil, nil, err
	}
	profilePath := profileFile.Name()
	cleanup := func() { _ = os.Remove(profilePath) }
	if err := profileFile.Chmod(0o600); err != nil {
		_ = profileFile.Close()
		cleanup()
		return nil, nil, err
	}
	if _, err := profileFile.WriteString(profile); err != nil {
		_ = profileFile.Close()
		cleanup()
		return nil, nil, err
	}
	if err := profileFile.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}
	args := []string{"-f", profilePath, executable, "worker"}
	args = append(args, workerArgs...)
	command := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", args...)
	command.Env = cleanEnvironment()
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return command, cleanup, nil
}

func macOSProfile(executable string, policy model.WorkerIsolation) (string, error) {
	statePath, err := filepath.EvalSymlinks(policy.StatePath)
	if err != nil {
		return "", err
	}
	temporaryPath, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", err
	}
	readPaths := []string{"/System", "/usr", "/bin", "/sbin", "/Library", "/private/etc", "/private/var/db", "/private/var/folders", "/dev", executable, statePath}
	readPaths = append(readPaths, policy.ReadPaths...)
	writePaths := []string{statePath, temporaryPath}
	writePaths = append(writePaths, policy.WritePaths...)
	var profile strings.Builder
	profile.WriteString("(version 1)\n(deny default)\n(import \"system.sb\")\n")
	profile.WriteString("(allow process-info* (target self))\n")
	profile.WriteString("(allow process-exec (literal ")
	profile.WriteString(strconv.Quote(canonicalPath(executable)))
	profile.WriteString("))\n(allow signal (target self))\n")
	profile.WriteString("(allow sysctl-read)\n(allow mach-lookup)\n(allow ipc-posix*)\n")
	profile.WriteString("(allow file-read-metadata)\n")
	for _, path := range uniquePaths(readPaths) {
		profile.WriteString("(allow file-read* (subpath ")
		profile.WriteString(strconv.Quote(path))
		profile.WriteString(") )\n")
	}
	for _, path := range uniquePaths(writePaths) {
		profile.WriteString("(allow file-write* (subpath ")
		profile.WriteString(strconv.Quote(path))
		profile.WriteString(") )\n")
	}
	if policy.AllowLocal {
		profile.WriteString("(allow network-inbound)\n")
	}
	if policy.AllowInternet {
		profile.WriteString("(allow network-outbound)\n")
	}
	return profile.String(), nil
}

func uniquePaths(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = canonicalPath(value)
		if value != "." && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func canonicalPath(value string) string {
	value = filepath.Clean(value)
	if absolute, err := filepath.Abs(value); err == nil {
		value = absolute
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		value = resolved
	}
	return value
}

func Enter([]string) error {
	return errors.New("nested macOS worker sandbox entry is not supported")
}

func ContainProcess(*os.Process) (func(), error) {
	return func() {}, nil
}

func Terminate(process *os.Process) error {
	if process == nil {
		return nil
	}
	if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("kill worker process group: %w", process.Kill())
	}
	return nil
}
