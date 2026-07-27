package desktop

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func configureDesktopLogin(preferences Preferences) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		path := filepath.Join(home, "Library", "LaunchAgents", "run.spare.desktop.plist")
		enabled := preferences.OpenAfterLogin || preferences.ShowInMenuBar
		if !enabled {
			_ = exec.Command("launchctl", "unload", path).Run()
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		content := darwinDesktopLoginContent(executable, preferences.OpenAfterLogin)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
		_ = exec.Command("launchctl", "unload", path).Run()
		return exec.Command("launchctl", "load", path).Run()
	case "windows":
		enabled := preferences.OpenAfterLogin || preferences.ShowInMenuBar
		if !enabled {
			_ = exec.Command("schtasks", "/Delete", "/TN", "Spare Desktop", "/F").Run()
			return nil
		}
		return exec.Command(
			"schtasks", "/Create", "/SC", "ONLOGON", "/TN", "Spare Desktop",
			"/TR", windowsDesktopLoginCommand(executable, preferences.OpenAfterLogin), "/F",
		).Run()
	default:
		config := os.Getenv("XDG_CONFIG_HOME")
		if config == "" {
			config = filepath.Join(home, ".config")
		}
		path := filepath.Join(config, "autostart", "spare-desktop.desktop")
		enabled := preferences.OpenAfterLogin || preferences.ShowInMenuBar
		if !enabled {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		content := linuxDesktopLoginContent(executable, preferences.OpenAfterLogin)
		return os.WriteFile(path, []byte(content), 0o600)
	}
}

func darwinDesktopLoginContent(executable string, openWindow bool) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(executable))
	arguments := "<string>" + escaped.String() + "</string>"
	if !openWindow {
		arguments += "<string>--hidden</string>"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>run.spare.desktop</string>
  <key>ProgramArguments</key><array>%s</array>
  <key>RunAtLoad</key><true/>
</dict></plist>
`, arguments)
}

func windowsDesktopLoginCommand(executable string, openWindow bool) string {
	command := `"` + executable + `"`
	if !openWindow {
		command += " --hidden"
	}
	return command
}

func linuxDesktopLoginContent(executable string, openWindow bool) string {
	command := fmt.Sprintf("%q", executable)
	if !openWindow {
		command += " --hidden"
	}
	return fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Spare\nExec=%s\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", command)
}
