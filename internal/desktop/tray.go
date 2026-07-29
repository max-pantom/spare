//go:build desktop

package desktop

import (
	"context"
	"fmt"
	"strings"

	"github.com/spare-run/spare/internal/model"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type trayController interface {
	Start(*App)
	Update(Snapshot)
	SetVisible(bool)
	Stop()
}

const (
	trayIconNeutral = iota
	trayIconReady
	trayIconWorking
	trayIconWarning
)

type trayPresentation struct {
	Headline       string
	Status         string
	OpenLabel      string
	ToggleLabel    string
	Address        string
	HasInstance    bool
	IsDrop         bool
	NeedsAttention bool
	CanReconnect   bool
	IconState      int
}

func (a *App) requestQuit() {
	a.stateMu.Lock()
	a.quitting = true
	a.stateMu.Unlock()
	wailsruntime.Quit(a.ctx)
}

func (a *App) handleTrayAction(action string) {
	switch action {
	case "open_recipe":
		a.stateMu.RLock()
		snapshot := a.last
		a.stateMu.RUnlock()
		if len(snapshot.Instances) > 0 && len(snapshot.Instances[0].URLs) > 0 {
			wailsruntime.BrowserOpenURL(a.ctx, snapshot.Instances[0].URLs[0])
		}
	case "toggle":
		a.stateMu.RLock()
		snapshot := a.last
		a.stateMu.RUnlock()
		if len(snapshot.Instances) > 0 {
			instance := snapshot.Instances[0]
			if instance.DesiredState == "running" {
				_, _ = a.StopInstance(instance.ID)
			} else {
				_, _ = a.StartInstance(instance.ID)
			}
		}
	case "open_files":
		a.stateMu.RLock()
		snapshot := a.last
		a.stateMu.RUnlock()
		if len(snapshot.Instances) > 0 && snapshot.Instances[0].DataPath != "" {
			_ = a.RevealPath(snapshot.Instances[0].DataPath)
		}
	case "copy_address":
		a.stateMu.RLock()
		presentation := presentTray(a.last)
		a.stateMu.RUnlock()
		if presentation.Address != "" {
			_ = wailsruntime.ClipboardSetText(a.ctx, presentation.Address)
		}
	case "share":
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "spare:navigate", "share")
	case "activity":
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "spare:navigate", "activity")
	case "settings", "reconnect":
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "spare:navigate", "settings")
	case "open_spare", "choose":
		a.showWindow()
		if action == "choose" {
			wailsruntime.EventsEmit(a.ctx, "spare:navigate", "recipes")
		}
	case "quit":
		a.requestQuit()
	}
}

func presentTray(snapshot Snapshot) trayPresentation {
	result := trayPresentation{
		Headline:  "No job",
		OpenLabel: "Choose a job",
		IconState: trayIconNeutral,
	}
	if len(snapshot.Instances) == 0 {
		return result
	}

	instance := snapshot.Instances[0]
	title := instance.RecipeID
	for _, recipe := range snapshot.Recipes {
		if recipe.ID == instance.RecipeID {
			title = recipe.Title
			break
		}
	}

	result.Headline = title
	result.HasInstance = true
	result.IsDrop = instance.RecipeID == model.RecipeDrop
	result.Address = preferredTrayAddress(instance.URLs)
	result.OpenLabel = "Open " + title
	if result.IsDrop {
		result.OpenLabel = "Open received files"
	}
	if instance.DesiredState == model.DesiredRunning {
		result.ToggleLabel = "Pause " + title
	} else {
		result.ToggleLabel = "Start " + title
	}

	if transfer, ok := activeTrayTransfer(snapshot.Events, instance.ID); ok {
		result.Headline = "Receiving " + transfer.name
		result.Status = fmt.Sprintf("%d%%", transfer.progress)
		result.IconState = trayIconWorking
		return result
	}

	switch instance.Status {
	case model.StatusHealthy:
		result.Status = "Ready"
		result.IconState = trayIconReady
	case model.StatusStarting, model.StatusRemoving:
		result.Status = "Working"
		result.IconState = trayIconWorking
	case model.StatusDegraded, model.StatusFailed:
		result.Headline = title + " needs attention"
		result.Status = "Check the job in Spare"
		result.NeedsAttention = true
		result.IconState = trayIconWarning
		if instance.Problem != nil {
			result.Status = strings.TrimSuffix(instance.Problem.Summary, ".")
			code := strings.ToLower(instance.Problem.Code)
			result.CanReconnect = strings.Contains(code, "folder") ||
				strings.Contains(code, "storage")
		}
	case model.StatusStopped:
		result.Status = "Stopped"
		result.IconState = trayIconNeutral
	default:
		result.Status = "Ready"
		result.IconState = trayIconNeutral
	}
	return result
}

type trayTransfer struct {
	name     string
	progress int
}

func activeTrayTransfer(events []model.Event, instanceID string) (trayTransfer, bool) {
	for _, event := range events {
		if event.InstanceID != instanceID ||
			(event.Kind != "drop_file_receiving" && event.Kind != "file_receiving") {
			continue
		}
		name, _ := event.Details["itemName"].(string)
		if name == "" {
			name, _ = event.Details["name"].(string)
		}
		progress, ok := trayProgress(event.Details["progress"])
		if name != "" && ok && progress >= 0 && progress < 100 {
			return trayTransfer{name: name, progress: progress}, true
		}
	}
	return trayTransfer{}, false
}

func trayProgress(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int64:
		return int(number), true
	case float64:
		return int(number), true
	case float32:
		return int(number), true
	default:
		return 0, false
	}
}

func preferredTrayAddress(urls []string) string {
	for _, value := range urls {
		if !strings.Contains(value, "127.0.0.1") &&
			!strings.Contains(value, ".local") {
			return value
		}
	}
	if len(urls) > 0 {
		return urls[0]
	}
	return ""
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (a *App) confirmTemporaryQuit(ctx context.Context) bool {
	a.stateMu.RLock()
	snapshot := a.last
	a.stateMu.RUnlock()
	var temporary *model.Instance
	for index := range snapshot.Instances {
		if snapshot.Instances[index].Mode == model.ModeTemporary {
			temporary = &snapshot.Instances[index]
			break
		}
	}
	if temporary == nil {
		return false
	}
	title := temporary.RecipeID
	for _, recipe := range snapshot.Recipes {
		if recipe.ID == temporary.RecipeID {
			title = recipe.Title
		}
	}
	choice, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         title + " is running temporarily",
		Message:       "Choose what happens to " + title + " before Spare quits.",
		Buttons:       []string{"Stop " + title + " and quit", "Keep " + title + " running", "Cancel"},
		DefaultButton: "Stop " + title + " and quit",
		CancelButton:  "Cancel",
	})
	if err != nil || choice == "Cancel" {
		a.stateMu.Lock()
		a.quitting = false
		a.stateMu.Unlock()
		return true
	}
	client, connectErr := a.ensureConnected(ctx)
	if connectErr != nil {
		if choice == "Keep "+title+" running" {
			_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Type:    wailsruntime.ErrorDialog,
				Title:   "Unable to keep " + title + " running",
				Message: "Spare could not reach its background service. Run repair, then try quitting again.",
			})
			a.stateMu.Lock()
			a.quitting = false
			a.stateMu.Unlock()
			return true
		}
		return false
	}
	if choice == "Keep "+title+" running" {
		if _, err := client.Promote(ctx, temporary.ID); err != nil {
			_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Type:    wailsruntime.ErrorDialog,
				Title:   "Unable to keep " + title + " running",
				Message: err.Error(),
			})
			a.stateMu.Lock()
			a.quitting = false
			a.stateMu.Unlock()
			return true
		}
	} else {
		_ = client.Remove(ctx, temporary.ID)
	}
	return false
}
