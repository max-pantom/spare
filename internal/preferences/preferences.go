package preferences

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Desktop contains preferences owned by the local desktop surface. The daemon
// reads KeepRecipesRunningAfterLogin so this setting still applies when the
// full desktop window is not launched after login.
type Desktop struct {
	Notifications                bool            `json:"notifications"`
	RecipeNotifications          map[string]bool `json:"recipeNotifications"`
	OpenAfterLogin               bool            `json:"openAfterLogin"`
	ShowInMenuBar                bool            `json:"showInMenuBar"`
	KeepRecipesRunningAfterLogin bool            `json:"keepRecipesRunningAfterLogin"`
}

func Defaults() Desktop {
	return Desktop{
		Notifications: true,
		RecipeNotifications: map[string]bool{
			"drop": true,
			"site": true,
			"hook": true,
		},
		ShowInMenuBar:                true,
		KeepRecipesRunningAfterLogin: true,
	}
}

func Load(root string) Desktop {
	result := Defaults()
	data, err := os.ReadFile(filepath.Join(root, "desktop.json"))
	if err != nil {
		return result
	}
	_ = json.Unmarshal(data, &result)
	result = normalize(result)
	return result
}

func Save(root string, preferences Desktop) error {
	preferences = normalize(preferences)
	data, err := json.MarshalIndent(preferences, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	target := filepath.Join(root, "desktop.json")
	temporary := target + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Chmod(target, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func normalize(value Desktop) Desktop {
	defaults := Defaults()
	if value.RecipeNotifications == nil {
		value.RecipeNotifications = defaults.RecipeNotifications
		return value
	}
	for recipeID, enabled := range defaults.RecipeNotifications {
		if _, exists := value.RecipeNotifications[recipeID]; !exists {
			value.RecipeNotifications[recipeID] = enabled
		}
	}
	return value
}

func NotificationsEnabled(value Desktop, recipeID string) bool {
	if !value.Notifications {
		return false
	}
	enabled, exists := value.RecipeNotifications[recipeID]
	return !exists || enabled
}
