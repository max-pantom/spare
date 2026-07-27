package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spare-run/spare/internal/paths"
)

func startupServiceCheck(_ paths.Paths) Check {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return definitionCheck(filepath.Join(home, "Library", "LaunchAgents", "run.spare.spared.plist"))
	case "linux":
		configRoot := os.Getenv("XDG_CONFIG_HOME")
		if configRoot == "" {
			home, _ := os.UserHomeDir()
			configRoot = filepath.Join(home, ".config")
		}
		return definitionCheck(filepath.Join(configRoot, "systemd", "user", "spared.service"))
	case "windows":
		if err := exec.Command("schtasks", "/Query", "/TN", "Spare").Run(); err != nil {
			return Check{
				ID:       "startup",
				Name:     "Login service",
				Status:   "failed",
				Message:  "The Spare logon task is missing.",
				Recovery: "Run `spare init` to register it again.",
			}
		}
		return Check{ID: "startup", Name: "Login service", Status: "healthy", Message: "The Spare logon task is registered."}
	default:
		return Check{ID: "startup", Name: "Login service", Status: "warning", Message: "This operating system is not supported."}
	}
}

func definitionCheck(path string) Check {
	if _, err := os.Stat(path); err != nil {
		return Check{
			ID:       "startup",
			Name:     "Login service",
			Status:   "failed",
			Message:  "The Spare login service definition is missing.",
			Recovery: "Run `spare init` to register it again.",
		}
	}
	return Check{ID: "startup", Name: "Login service", Status: "healthy", Message: "The Spare login service is registered."}
}
