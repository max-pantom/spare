package paths

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Paths struct {
	Root         string
	Database     string
	Token        string
	Endpoint     string
	Logs         string
	InstallState string
}

type Endpoint struct {
	URL       string    `json:"url"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
}

func Resolve() (Paths, error) {
	if override := os.Getenv("SPARE_HOME"); override != "" {
		return FromRoot(override), nil
	}

	var root string
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
		root = filepath.Join(home, "Library", "Application Support", "Spare")
	case "windows":
		root = os.Getenv("LOCALAPPDATA")
		if root == "" {
			return Paths{}, errors.New("LOCALAPPDATA is not set")
		}
		root = filepath.Join(root, "Spare")
	default:
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return Paths{}, err
			}
			state = filepath.Join(home, ".local", "state")
		}
		root = filepath.Join(state, "spare")
	}
	return FromRoot(root), nil
}

func FromRoot(root string) Paths {
	return Paths{
		Root:         root,
		Database:     filepath.Join(root, "spare.db"),
		Token:        filepath.Join(root, "api-token"),
		Endpoint:     filepath.Join(root, "endpoint.json"),
		Logs:         filepath.Join(root, "logs"),
		InstallState: filepath.Join(root, "install.json"),
	}
}

func (p Paths) Ensure() error {
	if err := os.MkdirAll(p.Logs, 0o700); err != nil {
		return fmt.Errorf("create Spare state directory: %w", err)
	}
	return os.Chmod(p.Root, 0o700)
}

func (p Paths) WriteEndpoint(endpoint Endpoint) error {
	data, err := json.Marshal(endpoint)
	if err != nil {
		return err
	}
	tmp := p.Endpoint + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.Endpoint)
}

func (p Paths) ReadEndpoint() (Endpoint, error) {
	data, err := os.ReadFile(p.Endpoint)
	if err != nil {
		return Endpoint{}, err
	}
	var endpoint Endpoint
	if err := json.Unmarshal(data, &endpoint); err != nil {
		return Endpoint{}, err
	}
	return endpoint, nil
}
