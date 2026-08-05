//go:build desktop

package desktop

import (
	"fmt"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/preferences"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) notify(event model.Event) {
	recipeID := ""
	if event.InstanceID != "" {
		a.stateMu.RLock()
		for _, instance := range a.last.Instances {
			if instance.ID == event.InstanceID {
				recipeID = instance.RecipeID
				break
			}
		}
		a.stateMu.RUnlock()
	}
	monitorTransition := recipeID == model.RecipeMonitor &&
		(event.Kind == "instance_degraded" || event.Kind == "instance_recovered")
	if event.Kind != "drop_file_received" &&
		event.Level != "error" &&
		event.Kind != "worker_exited" &&
		event.Kind != "hook_request_captured" &&
		!monitorTransition {
		return
	}
	if !preferences.NotificationsEnabled(loadPreferences(a.paths.Root), recipeID) {
		return
	}
	title := "Spare needs attention"
	if event.Kind == "drop_file_received" {
		title = "File received"
	} else if event.Kind == "hook_request_captured" {
		title = "Hook captured a request"
	} else if recipeID == model.RecipeMonitor && event.Kind == "instance_degraded" {
		title = "Monitor found a problem"
	} else if recipeID == model.RecipeMonitor && event.Kind == "instance_recovered" {
		title = "Monitor recovered"
	}
	_ = wailsruntime.InitializeNotifications(a.ctx)
	_ = wailsruntime.SendNotification(a.ctx, wailsruntime.NotificationOptions{
		ID:    fmt.Sprintf("spare-%d", event.ID),
		Title: title,
		Body:  event.Message,
		Data:  map[string]any{"eventId": event.ID},
	})
}
