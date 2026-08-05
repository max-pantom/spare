//go:build darwin

package isolation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spare-run/spare/internal/model"
)

func TestMacOSSandboxAllowsDeclaredReadAndDeniesOtherFiles(t *testing.T) {
	root := t.TempDir()
	allowedDir := filepath.Join(root, "allowed")
	stateDir := filepath.Join(root, "state")
	for _, path := range []string{allowedDir, stateDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	allowed := filepath.Join(allowedDir, "visible.txt")
	denied, err := filepath.Abs("environment.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowed, []byte("visible"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := macOSProfile("/bin/cat", model.WorkerIsolation{StatePath: stateDir, ReadPaths: []string{allowedDir}})
	if err != nil {
		t.Fatal(err)
	}
	allowedCommand := exec.Command("/usr/bin/sandbox-exec", "-p", profile, "/bin/cat", allowed)
	allowedCommand.Env = cleanEnvironment()
	output, err := allowedCommand.CombinedOutput()
	if err != nil || string(output) != "visible" {
		t.Fatalf("declared read failed: %q, %v", output, err)
	}
	deniedCommand := exec.Command("/usr/bin/sandbox-exec", "-p", profile, "/bin/cat", denied)
	deniedCommand.Env = cleanEnvironment()
	if output, err := deniedCommand.CombinedOutput(); err == nil {
		t.Fatalf("undeclared read succeeded: %q", output)
	}
	otherExecutable := exec.Command("/usr/bin/sandbox-exec", "-p", profile, "/bin/echo", "unexpected")
	otherExecutable.Env = cleanEnvironment()
	if output, err := otherExecutable.CombinedOutput(); err == nil {
		t.Fatalf("undeclared executable started: %q", output)
	}
}
