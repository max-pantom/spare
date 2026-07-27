package doctor

import (
	"context"
	"os"
	"strings"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
)

type Check struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Recovery string `json:"recovery,omitempty"`
}

type Report struct {
	Checks []Check `json:"checks"`
}

func Run(ctx context.Context, client *api.Client, statePaths paths.Paths) Report {
	report := Report{}
	if client == nil {
		report.Checks = append(report.Checks, Check{
			ID:       "daemon",
			Name:     "Daemon",
			Status:   "failed",
			Message:  "Spare is not running.",
			Recovery: "Run `spare init` to start it.",
		})
		return report
	}
	if err := client.Health(ctx); err != nil {
		report.Checks = append(report.Checks, Check{
			ID:       "daemon",
			Name:     "Daemon",
			Status:   "failed",
			Message:  err.Error(),
			Recovery: "Run `spare init`, then check the daemon log.",
		})
		return report
	}
	report.Checks = append(report.Checks,
		Check{ID: "daemon", Name: "Daemon", Status: "healthy", Message: "The local management service is reachable."},
		Check{ID: "dashboard", Name: "Dashboard", Status: "healthy", Message: "The authenticated dashboard API is reachable."},
	)
	machine, machineErr := client.Machine(ctx)
	if machineErr == nil && machine.Capabilities.HasBattery {
		report.Checks = append(report.Checks, Check{
			ID:       "sleep",
			Name:     "Computer sleep",
			Status:   "warning",
			Message:  "This computer has a battery and may sleep when its lid is closed.",
			Recovery: sleepRecovery(machine.OS),
		})
	} else if machineErr == nil {
		report.Checks = append(report.Checks, Check{
			ID:      "sleep",
			Name:    "Computer sleep",
			Status:  "ready",
			Message: "No laptop battery was detected.",
		})
	}
	report.Checks = append(report.Checks, startupServiceCheck(statePaths))
	instances, err := client.Instances(ctx)
	if err != nil {
		report.Checks = append(report.Checks, Check{
			ID:       "instances",
			Name:     "Recipes",
			Status:   "failed",
			Message:  err.Error(),
			Recovery: "Restart Spare and run `spare doctor` again.",
		})
		return report
	}
	if len(instances) == 0 {
		report.Checks = append(report.Checks, Check{
			ID:      "instance",
			Name:    "Recipe",
			Status:  "ready",
			Message: "No recipe is installed.",
		})
		return report
	}
	for _, instance := range instances {
		report.Checks = append(report.Checks, instanceChecks(instance)...)
	}
	return report
}

func instanceChecks(instance model.Instance) []Check {
	title := recipeTitle(instance.RecipeID)
	result := []Check{{
		ID:      "instance." + instance.ID,
		Name:    title,
		Status:  instance.Status,
		Message: instanceMessage(instance, title),
	}}
	if instance.Problem != nil {
		result[0].Recovery = instance.Problem.Recovery
	}
	if instance.DataPath != "" {
		result = append(result, storageChecks(instance)...)
	}
	if instance.Status == model.StatusHealthy || instance.Status == model.StatusStarting || instance.Status == model.StatusDegraded {
		result = append(result, networkChecks(instance)...)
	}
	return result
}

func instanceMessage(instance model.Instance, title string) string {
	if instance.Problem != nil {
		return instance.Problem.Summary
	}
	switch instance.Status {
	case model.StatusHealthy:
		return title + " is healthy."
	case model.StatusStarting:
		return title + " is starting."
	case model.StatusStopped:
		return title + " is stopped."
	default:
		return title + " is " + instance.Status + "."
	}
}

func recipeTitle(id string) string {
	if id == "" {
		return "Recipe"
	}
	return strings.ToUpper(id[:1]) + id[1:]
}

func sleepRecovery(goos string) string {
	switch goos {
	case "darwin":
		return "Keep the Mac awake while the recipe must remain available and review Battery settings."
	case "windows":
		return "Keep the PC awake while the recipe must remain available and review Power & battery settings."
	default:
		return "Keep the computer awake while the recipe must remain available and review its power settings."
	}
}

func writable(path string) bool {
	probe, err := os.CreateTemp(path, ".spare-doctor-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	return closeErr == nil && removeErr == nil
}
