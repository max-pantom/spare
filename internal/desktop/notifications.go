//go:build desktop

package desktop

import (
	"fmt"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/preferences"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) notify(event model.Event) {
	if event.Kind != "drop_file_received" &&
		event.Level != "error" &&
		event.Kind != "worker_exited" &&
		event.Kind != "hook_request_captured" {
		return
	}
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
	if !preferences.NotificationsEnabled(loadPreferences(a.paths.Root), recipeID) {
		return
	}
	title := "Spare needs attention"
	if event.Kind == "drop_file_received" {
		title = "File received"
	} else if event.Kind == "hook_request_captured" {
		title = "Hook captured a request"
	}
	_ = wailsruntime.InitializeNotifications(a.ctx)
	_ = wailsruntime.SendNotification(a.ctx, wailsruntime.NotificationOptions{
		ID:    fmt.Sprintf("spare-%d", event.ID),
		Title: title,
		Body:  event.Message,
		Data:  map[string]any{"eventId": event.ID},
	})
}
