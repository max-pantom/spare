//go:build desktop

package desktop

import (
	"errors"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/recipeview"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ChooseDirectory(purpose string) (string, error) {
	title := "Choose a folder"
	switch purpose {
	case "drop":
		title = "Choose where Drop saves received files"
	case "site":
		title = "Choose the folder Site serves"
	case "restore":
		title = "Choose where Spare restores the backup"
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                title,
		CanCreateDirectories: true,
		ResolvesAliases:      true,
	})
}

func (a *App) ChooseFile(purpose string) (string, error) {
	options := wailsruntime.OpenDialogOptions{Title: "Choose a file"}
	switch purpose {
	case "backup":
		options.Title = "Choose a Spare backup"
		options.Filters = []wailsruntime.FileFilter{{
			DisplayName: "Spare backups (*.spare-backup)",
			Pattern:     "*.spare-backup",
		}}
	case "recipe":
		options.Title = "Choose a Spare recipe package"
		options.Filters = []wailsruntime.FileFilter{{
			DisplayName: "Spare recipes (*.sp)",
			Pattern:     "*.sp",
		}}
	}
	return wailsruntime.OpenFileDialog(a.ctx, options)
}

func (a *App) ChooseFiles(purpose string) ([]string, error) {
	title := "Choose files"
	if purpose == "drop" {
		title = "Choose files to add to Drop"
	}
	return wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: title,
	})
}

func (a *App) DescribeDroppedPaths(paths []string) ([]DroppedPath, error) {
	return describeDroppedPaths(paths)
}

func (a *App) OpenRecipePackage(source string) error {
	if source == "" {
		selected, err := a.ChooseFile("recipe")
		if err != nil || selected == "" {
			return err
		}
		source = selected
	}
	viewer, err := recipeview.New(source)
	if err != nil {
		return err
	}
	running, err := viewer.Start()
	if err != nil {
		return err
	}
	wailsruntime.BrowserOpenURL(a.ctx, running.URL)
	go func() {
		_ = running.Wait(a.ctx)
	}()
	return nil
}

func (a *App) AddDropFiles(instanceID string, sources []string) ([]string, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return nil, err
	}
	names, err := client.AddDropFiles(a.ctx, instanceID, sources)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return names, err
}

func (a *App) ExportBackup(instanceID string) (string, error) {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return "", err
	}
	instance, err := client.Instance(a.ctx, instanceID)
	if err != nil {
		return "", err
	}
	destination, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export a Spare backup",
		DefaultFilename: instance.RecipeID + ".spare-backup",
		Filters: []wailsruntime.FileFilter{{
			DisplayName: "Spare backups (*.spare-backup)",
			Pattern:     "*.spare-backup",
		}},
	})
	if err != nil || destination == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(destination), ".spare-backup") {
		destination += ".spare-backup"
	}
	if err := client.ExportBackup(a.ctx, instanceID, destination); err != nil {
		return "", err
	}
	_, _ = a.Snapshot()
	return destination, nil
}

func (a *App) RestoreBackup(source string) (model.Instance, error) {
	if source == "" {
		selected, err := a.ChooseFile("backup")
		if err != nil || selected == "" {
			return model.Instance{}, err
		}
		source = selected
	}
	destination, err := a.ChooseDirectory("restore")
	if err != nil || destination == "" {
		return model.Instance{}, err
	}
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return model.Instance{}, err
	}
	instance, err := client.RestoreBackup(a.ctx, source, destination)
	if err == nil {
		_, _ = a.Snapshot()
	}
	return instance, err
}

func (a *App) OpenURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("choose a valid Spare HTTP address")
	}
	snapshot, err := a.Snapshot()
	if err != nil {
		return err
	}
	allowed := false
	for _, instance := range snapshot.Instances {
		for _, candidate := range instance.URLs {
			if strings.TrimRight(value, "/") == strings.TrimRight(candidate, "/") {
				allowed = true
			}
		}
	}
	if !allowed {
		return errors.New("this address does not belong to the active Spare recipe")
	}
	wailsruntime.BrowserOpenURL(a.ctx, value)
	return nil
}

// OpenDashboard creates the same single-use browser session as
// `spare open dashboard`. The long-lived bearer token remains in Go and the
// short-lived exchange code is handed directly to the operating-system
// browser, never to the React application.
func (a *App) OpenDashboard() error {
	client, err := a.ensureConnected(a.ctx)
	if err != nil {
		return err
	}
	dashboardURL, err := client.BrowserSession(a.ctx)
	if err != nil {
		return err
	}
	wailsruntime.BrowserOpenURL(a.ctx, dashboardURL)
	return nil
}

func (a *App) RevealPath(path string) error {
	snapshot, err := a.Snapshot()
	if err != nil {
		return err
	}
	allowed := false
	for _, instance := range snapshot.Instances {
		if instance.DataPath != "" && filepath.Clean(instance.DataPath) == filepath.Clean(path) {
			allowed = true
		}
	}
	if !allowed {
		return errors.New("this folder does not belong to the active Spare recipe")
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", path)
	case "windows":
		command = exec.Command("explorer", path)
	default:
		command = exec.Command("xdg-open", path)
	}
	return command.Start()
}

func (a *App) RevealReceivedFile(instanceID, name string) error {
	snapshot, err := a.Snapshot()
	if err != nil {
		return err
	}
	root := ""
	for _, instance := range snapshot.Instances {
		if instance.ID == instanceID && instance.RecipeID == model.RecipeDrop {
			root = instance.DataPath
			break
		}
	}
	if root == "" {
		return errors.New("this file does not belong to the active Drop")
	}
	target, err := resolveRevealedItem(root, name)
	if err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", "-R", target)
	case "windows":
		command = exec.Command("explorer", "/select,"+target)
	default:
		command = exec.Command("xdg-open", filepath.Dir(target))
	}
	return command.Start()
}
