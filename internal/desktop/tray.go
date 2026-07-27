//go:build desktop

package desktop

import (
	"context"

	"github.com/spare-run/spare/internal/model"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type trayController interface {
	Start(*App)
	Update(Snapshot)
	SetVisible(bool)
	Stop()
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
	case "share":
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "spare:navigate", "share")
	case "activity":
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "spare:navigate", "activity")
	case "open_spare", "choose":
		a.showWindow()
		if action == "choose" {
			wailsruntime.EventsEmit(a.ctx, "spare:navigate", "recipes")
		}
	case "quit":
		a.requestQuit()
	}
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
