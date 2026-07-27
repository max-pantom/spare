package desktop

import (
	"github.com/spare-run/spare/internal/preferences"
)

func loadPreferences(root string) Preferences {
	return preferences.Load(root)
}

func savePreferences(root string, values Preferences) error {
	return preferences.Save(root, values)
}
