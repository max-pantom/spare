package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Definition struct {
	Path    string
	Content string
	Name    string
}

func BuildDefinition(goos, executable, stateRoot, home string) (Definition, error) {
	switch goos {
	case "darwin":
		path := filepath.Join(home, "Library", "LaunchAgents", "run.spare.spared.plist")
		content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>run.spare.spared</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, escapeXML(executable), escapeXML(filepath.Join(stateRoot, "logs", "spared.log")), escapeXML(filepath.Join(stateRoot, "logs", "spared.log")))
		return Definition{Path: path, Content: content, Name: "run.spare.spared"}, nil
	case "linux":
		configRoot := os.Getenv("XDG_CONFIG_HOME")
		if configRoot == "" {
			configRoot = filepath.Join(home, ".config")
		}
		path := filepath.Join(configRoot, "systemd", "user", "spared.service")
		content := fmt.Sprintf(`[Unit]
Description=Spare local utility runtime
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=2
Environment=SPARE_HOME=%s

[Install]
WantedBy=default.target
`, systemdQuote(executable), systemdQuote(stateRoot))
		return Definition{Path: path, Content: content, Name: "spared.service"}, nil
	case "windows":
		return Definition{
			Name:    "Spare",
			Content: fmt.Sprintf(`"%s"`, executable),
		}, nil
	default:
		return Definition{}, fmt.Errorf("unsupported operating system: %s", goos)
	}
}

func InstallAndStart(ctx context.Context, executable, stateRoot string) error {
	if os.Getenv("SPARE_NO_SERVICE") == "1" {
		return startDetached(executable, stateRoot)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	definition, err := BuildDefinition(runtime.GOOS, executable, stateRoot, home)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		if err := writeDefinition(definition); err != nil {
			return err
		}
		uid, err := currentUID()
		if err != nil {
			return err
		}
		domain := "gui/" + strconv.Itoa(uid)
		_ = exec.CommandContext(ctx, "launchctl", "bootout", domain, definition.Path).Run()
		if output, err := exec.CommandContext(ctx, "launchctl", "bootstrap", domain, definition.Path).CombinedOutput(); err != nil {
			return fmt.Errorf("register Spare login service: %s: %w", strings.TrimSpace(string(output)), err)
		}
		if output, err := exec.CommandContext(ctx, "launchctl", "kickstart", "-k", domain+"/"+definition.Name).CombinedOutput(); err != nil {
			return fmt.Errorf("start Spare login service: %s: %w", strings.TrimSpace(string(output)), err)
		}
	case "linux":
		if err := writeDefinition(definition); err != nil {
			return err
		}
		if output, err := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
			return fmt.Errorf("reload user services: %s: %w", strings.TrimSpace(string(output)), err)
		}
		if output, err := exec.CommandContext(ctx, "systemctl", "--user", "enable", "--now", definition.Name).CombinedOutput(); err != nil {
			return fmt.Errorf("start Spare login service: %s: %w", strings.TrimSpace(string(output)), err)
		}
	case "windows":
		taskCommand := fmt.Sprintf(`"%s"`, executable)
		args := []string{"/Create", "/SC", "ONLOGON", "/TN", definition.Name, "/TR", taskCommand, "/F"}
		if output, err := exec.CommandContext(ctx, "schtasks", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("register Spare login task: %s: %w", strings.TrimSpace(string(output)), err)
		}
		if output, err := exec.CommandContext(ctx, "schtasks", "/Run", "/TN", definition.Name).CombinedOutput(); err != nil {
			return fmt.Errorf("start Spare login task: %s: %w", strings.TrimSpace(string(output)), err)
		}
	}
	return nil
}

func Uninstall(ctx context.Context, stateRoot string) error {
	if os.Getenv("SPARE_NO_SERVICE") == "1" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	definition, err := BuildDefinition(runtime.GOOS, "", stateRoot, home)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		uid, err := currentUID()
		if err != nil {
			return err
		}
		domain := "gui/" + strconv.Itoa(uid)
		_ = exec.CommandContext(ctx, "launchctl", "bootout", domain, definition.Path).Run()
		if err := os.Remove(definition.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	case "linux":
		_ = exec.CommandContext(ctx, "systemctl", "--user", "disable", "--now", definition.Name).Run()
		if err := os.Remove(definition.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
	case "windows":
		_ = exec.CommandContext(ctx, "schtasks", "/End", "/TN", definition.Name).Run()
		if output, err := exec.CommandContext(ctx, "schtasks", "/Delete", "/TN", definition.Name, "/F").CombinedOutput(); err != nil {
			return fmt.Errorf("remove Spare login task: %s: %w", strings.TrimSpace(string(output)), err)
		}
	}
	return nil
}

func writeDefinition(definition Definition) error {
	if err := os.MkdirAll(filepath.Dir(definition.Path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(definition.Path, []byte(definition.Content), 0o600)
}

func startDetached(executable, stateRoot string) error {
	command := exec.Command(executable)
	command.Env = append(os.Environ(), "SPARE_HOME="+stateRoot)
	logPath := filepath.Join(stateRoot, "logs", "spared.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = command.Process.Release()
	return logFile.Close()
}

func escapeXML(value string) string {
	var output bytes.Buffer
	_ = xml.EscapeText(&output, []byte(value))
	return output.String()
}

func systemdQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func currentUID() (int, error) {
	current, err := user.Current()
	if err != nil {
		return 0, err
	}
	uid, err := strconv.Atoi(current.Uid)
	if err != nil {
		return 0, fmt.Errorf("read current user ID: %w", err)
	}
	return uid, nil
}
