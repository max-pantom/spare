package paths

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type Paths struct {
	Root         string
	Database     string
	Token        string
	Endpoint     string
	Logs         string
	JobPackages  string
	JobData      string
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
		JobPackages:  filepath.Join(root, "job-packages"),
		JobData:      filepath.Join(root, "jobs"),
		InstallState: filepath.Join(root, "install.json"),
	}
}

func (p Paths) Ensure() error {
	rootInfo, err := os.Lstat(p.Root)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(p.Root, 0o700); err != nil {
			return fmt.Errorf("create Spare state root: %w", err)
		}
		rootInfo, err = os.Lstat(p.Root)
	}
	if err != nil {
		return fmt.Errorf("inspect Spare state root: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("the Spare state root is not a private directory")
	}
	if err := os.Chmod(p.Root, 0o700); err != nil {
		return fmt.Errorf("secure Spare state root: %w", err)
	}
	for _, directory := range []string{p.Logs, p.JobPackages, p.JobData} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create Spare state directory: %w", err)
		}
		info, err := os.Lstat(directory)
		if err != nil {
			return fmt.Errorf("inspect Spare state directory: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("a Spare state path is not a private directory")
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return fmt.Errorf("secure Spare state directory: %w", err)
		}
	}
	return nil
}

func (p Paths) WriteEndpoint(endpoint Endpoint) error {
	if err := validateEndpoint(endpoint); err != nil {
		return err
	}
	data, err := json.Marshal(endpoint)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(p.Endpoint), ".spare-endpoint-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return atomicReplace(temporaryPath, p.Endpoint)
}

func (p Paths) ReadEndpoint() (Endpoint, error) {
	info, err := os.Lstat(p.Endpoint)
	if err != nil {
		return Endpoint{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return Endpoint{}, errors.New("the Spare endpoint file is invalid")
	}
	file, err := os.Open(p.Endpoint)
	if err != nil {
		return Endpoint{}, err
	}
	defer file.Close()
	var endpoint Endpoint
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&endpoint); err != nil {
		return Endpoint{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Endpoint{}, errors.New("the Spare endpoint file contains trailing data")
	}
	if err := validateEndpoint(endpoint); err != nil {
		return Endpoint{}, err
	}
	if err := os.Chmod(p.Endpoint, 0o600); err != nil {
		return Endpoint{}, err
	}
	return endpoint, nil
}

func validateEndpoint(endpoint Endpoint) error {
	parsed, err := url.Parse(endpoint.URL)
	if err != nil || parsed == nil {
		return errors.New("the Spare endpoint is invalid")
	}
	port, portErr := strconv.Atoi(parsed.Port())
	if portErr != nil ||
		port < 7331 || port > 7339 ||
		parsed.Scheme != "http" ||
		parsed.User != nil ||
		parsed.Hostname() != "127.0.0.1" ||
		parsed.Port() == "" ||
		parsed.Path != "" ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		endpoint.PID <= 0 ||
		endpoint.StartedAt.IsZero() {
		return errors.New("the Spare endpoint is invalid")
	}
	return nil
}
