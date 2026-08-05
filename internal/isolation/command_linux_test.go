//go:build linux

package isolation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spare-run/spare/internal/model"
)

func TestLandlockAllowsDeclaredReadAndDeniesSiblingFile(t *testing.T) {
	if os.Getenv("SPARE_LANDLOCK_TEST_HELPER") == "1" {
		allowed, denied, state := os.Args[len(os.Args)-3], os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
		if err := applyLandlock(model.WorkerIsolation{StatePath: state, ReadPaths: []string{allowed}}); err != nil {
			os.Exit(2)
		}
		if _, err := os.ReadFile(allowed); err != nil {
			os.Exit(3)
		}
		if _, err := os.ReadFile(denied); err == nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	allowed, err := filepath.Abs("environment.go")
	if err != nil {
		t.Fatal(err)
	}
	denied, err := filepath.Abs("command_linux.go")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=TestLandlockAllowsDeclaredReadAndDeniesSiblingFile", "--", allowed, denied, t.TempDir())
	command.Env = append(os.Environ(), "SPARE_LANDLOCK_TEST_HELPER=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Landlock helper failed: %q, %v", output, err)
	}
}
