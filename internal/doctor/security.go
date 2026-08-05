package doctor

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/artifacts"
	"github.com/spare-run/spare/internal/paths"
)

func RunSecurity(ctx context.Context, client *api.Client, statePaths paths.Paths) Report {
	report := Report{Checks: []Check{
		privateStateCheck(statePaths),
		endpointSecurityCheck(statePaths),
		serviceSecurityCheck(statePaths),
		executableIntegrityCheck(),
		workerIsolationCheck(),
	}}
	if client == nil {
		report.Checks = append(report.Checks, Check{
			ID:       "security.packages",
			Name:     "Job packages",
			Status:   "warning",
			Message:  "Spare is not running, so installed package signatures could not be checked.",
			Recovery: "Start Spare and run `spare doctor --security` again.",
		})
		return report
	}
	packages, err := client.JobPackages(ctx)
	if err != nil {
		report.Checks = append(report.Checks, Check{
			ID:       "security.packages",
			Name:     "Job packages",
			Status:   "failed",
			Message:  "Installed job package signatures could not be checked.",
			Recovery: err.Error(),
		})
	} else {
		invalid := 0
		for _, candidate := range packages {
			if candidate.SignatureStatus != "verified" {
				invalid++
			}
		}
		if invalid == 0 {
			report.Checks = append(report.Checks, Check{
				ID:      "security.packages",
				Name:    "Job packages",
				Status:  "healthy",
				Message: fmt.Sprintf("%d optional job package signatures are valid.", len(packages)),
			})
		} else {
			report.Checks = append(report.Checks, Check{
				ID:       "security.packages",
				Name:     "Job packages",
				Status:   "failed",
				Message:  fmt.Sprintf("%d installed job packages failed verification.", invalid),
				Recovery: "Do not start those jobs. Remove them and reinstall from the official catalog.",
			})
		}
	}
	instances, err := client.Instances(ctx)
	if err == nil {
		for _, instance := range instances {
			for _, address := range instance.URLs {
				parsed, parseErr := url.Parse(address)
				if parseErr == nil && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
					report.Checks = append(report.Checks, Check{
						ID:       "security.network." + instance.ID,
						Name:     recipeTitle(instance.RecipeID) + " network",
						Status:   "warning",
						Message:  "This job is reachable from the local network at " + address + ".",
						Recovery: "Use it only on a trusted network and do not forward this port through a router or tunnel.",
					})
					break
				}
			}
		}
	}
	return report
}

func privateStateCheck(statePaths paths.Paths) Check {
	if runtime.GOOS == "windows" {
		if err := paths.VerifyPrivateTree(statePaths.Root); err != nil {
			return Check{
				ID:       "security.state",
				Name:     "Private state",
				Status:   "failed",
				Message:  "Spare state is not restricted to the current Windows user.",
				Recovery: "Run `spare init` to replace inherited access with owner-only Windows ACLs.",
			}
		}
		return Check{
			ID:      "security.state",
			Name:    "Private state",
			Status:  "healthy",
			Message: "Credentials, logs, packages, and job data use a protected current-user Windows ACL.",
		}
	}
	for _, candidate := range []struct {
		path string
		mode os.FileMode
	}{
		{statePaths.Root, 0o700},
		{statePaths.Logs, 0o700},
		{statePaths.JobPackages, 0o700},
		{statePaths.JobData, 0o700},
		{statePaths.Token, 0o600},
	} {
		info, err := os.Lstat(candidate.path)
		if err != nil || info.Mode().Perm() != candidate.mode || info.Mode()&os.ModeSymlink != 0 {
			return Check{
				ID:       "security.state",
				Name:     "Private state",
				Status:   "failed",
				Message:  "A Spare state path is missing, linked, or accessible more broadly than intended.",
				Recovery: "Run `spare init` to repair private state permissions.",
			}
		}
	}
	return Check{
		ID:      "security.state",
		Name:    "Private state",
		Status:  "healthy",
		Message: "API credentials, logs, packages, and job state are owner-only.",
	}
}

func endpointSecurityCheck(statePaths paths.Paths) Check {
	endpoint, err := statePaths.ReadEndpoint()
	if err != nil {
		return Check{
			ID:       "security.control",
			Name:     "Control API",
			Status:   "warning",
			Message:  "The private loopback endpoint is unavailable or invalid.",
			Recovery: "Start Spare and run this check again.",
		}
	}
	return Check{
		ID:      "security.control",
		Name:    "Control API",
		Status:  "healthy",
		Message: fmt.Sprintf("The authenticated management API is bound to %s (PID %d).", endpoint.URL, endpoint.PID),
	}
}

func serviceSecurityCheck(statePaths paths.Paths) Check {
	if runtime.GOOS == "linux" {
		configRoot := os.Getenv("XDG_CONFIG_HOME")
		if configRoot == "" {
			home, _ := os.UserHomeDir()
			configRoot = filepath.Join(home, ".config")
		}
		definition := filepath.Join(configRoot, "systemd", "user", "spared.service")
		data, err := os.ReadFile(definition)
		if err == nil {
			value := string(data)
			for _, required := range []string{"NoNewPrivileges=true", "PrivateTmp=true", "MemoryDenyWriteExecute=true", "CapabilityBoundingSet="} {
				if !strings.Contains(value, required) {
					return Check{ID: "security.service", Name: "Login service", Status: "warning", Message: "The login service predates current hardening.", Recovery: "Run `spare init` to replace it."}
				}
			}
			return Check{ID: "security.service", Name: "Login service", Status: "healthy", Message: "The per-user systemd service includes process and kernel hardening."}
		}
	}
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		definition := filepath.Join(home, "Library", "LaunchAgents", "run.spare.spared.plist")
		data, err := os.ReadFile(definition)
		if err == nil && strings.Contains(string(data), "<key>Umask</key>") {
			return Check{ID: "security.service", Name: "Login service", Status: "healthy", Message: "The per-user LaunchAgent uses a private file-creation mask."}
		}
	}
	return Check{
		ID:       "security.service",
		Name:     "Login service",
		Status:   "warning",
		Message:  "The per-user login service could not be confirmed as hardened.",
		Recovery: "Run `spare init`, then run this check again.",
	}
}

func executableIntegrityCheck() Check {
	executable, err := os.Executable()
	if err != nil {
		return Check{ID: "security.executable", Name: "Executable", Status: "warning", Message: "The current Spare executable could not be identified."}
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return Check{ID: "security.executable", Name: "Executable", Status: "warning", Message: "The current Spare executable path could not be resolved."}
	}
	checksum, err := artifacts.SHA256(executable)
	if err != nil {
		return Check{ID: "security.executable", Name: "Executable", Status: "warning", Message: "The current Spare executable could not be hashed."}
	}
	return Check{
		ID:      "security.executable",
		Name:    "Executable",
		Status:  "ready",
		Message: executable + " · SHA-256 " + checksum,
	}
}

func workerIsolationCheck() Check {
	if runtime.GOOS == "darwin" {
		return Check{
			ID:      "security.isolation",
			Name:    "Worker isolation",
			Status:  "healthy",
			Message: "Built-in workers use a deny-by-default macOS sandbox with manifest-derived folder and network access.",
		}
	}
	if runtime.GOOS == "linux" {
		return Check{
			ID:      "security.isolation",
			Name:    "Worker isolation",
			Status:  "healthy",
			Message: "Built-in workers use Landlock filesystem isolation and a dedicated process group.",
		}
	}
	if runtime.GOOS == "windows" {
		return Check{
			ID:       "security.isolation",
			Name:     "Worker isolation",
			Status:   "warning",
			Message:  "Built-in workers use a restricted Job Object, but per-worker filesystem isolation is not complete.",
			Recovery: "Run only Spare-signed built-in jobs until AppContainer filesystem enforcement is added.",
		}
	}
	return Check{
		ID:       "security.isolation",
		Name:     "Worker isolation",
		Status:   "warning",
		Message:  "Built-in workers run without a supported OS filesystem sandbox on this platform.",
		Recovery: "Run only Spare-signed jobs on a trusted account while platform sandbox enforcement is being completed.",
	}
}
