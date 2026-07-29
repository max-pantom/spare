package preferences

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDesktopPreferencesDefaultAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	defaults := Load(root)
	if !defaults.Notifications || !defaults.ShowInMenuBar ||
		!defaults.KeepRecipesRunningAfterLogin || defaults.OpenAfterLogin ||
		defaults.Theme != "dark" {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}

	expected := Defaults()
	expected.Theme = "light"
	expected.OpenAfterLogin = true
	expected.KeepRecipesRunningAfterLogin = false
	expected.Notifications = false
	expected.ShowInMenuBar = false
	expected.RecipeNotifications["hook"] = false
	if err := Save(root, expected); err != nil {
		t.Fatal(err)
	}
	actual := Load(root)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("round trip = %#v, want %#v", actual, expected)
	}
	info, err := os.Stat(filepath.Join(root, "desktop.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
}

func TestDesktopPreferencesPreserveNewDefaultsForOlderFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "desktop.json"),
		[]byte(`{"notifications":false,"showInMenuBar":false}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	loaded := Load(root)
	if loaded.Notifications || loaded.ShowInMenuBar ||
		loaded.Theme != "dark" ||
		!loaded.KeepRecipesRunningAfterLogin ||
		!loaded.RecipeNotifications["drop"] ||
		!loaded.RecipeNotifications["site"] ||
		!loaded.RecipeNotifications["hook"] {
		t.Fatalf("older preference migration = %#v", loaded)
	}
}

func TestDesktopPreferencesNormalizeRemovedTheme(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "desktop.json"),
		[]byte(`{"theme":"clear"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if loaded := Load(root); loaded.Theme != "dark" {
		t.Fatalf("removed theme normalized to %q, want dark", loaded.Theme)
	}
}

func TestNotificationsEnabledByRecipe(t *testing.T) {
	value := Defaults()
	value.RecipeNotifications["drop"] = false
	if NotificationsEnabled(value, "drop") {
		t.Fatal("Drop notifications should be disabled")
	}
	if !NotificationsEnabled(value, "hook") {
		t.Fatal("Hook notifications should remain enabled")
	}
	value.Notifications = false
	if NotificationsEnabled(value, "hook") {
		t.Fatal("global notification switch should disable every recipe")
	}
}
