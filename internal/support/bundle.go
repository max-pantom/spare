package support

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/doctor"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
)

const Schema = "spare.support/v1"

type Bundle struct {
	Schema       string         `json:"schema"`
	GeneratedAt  time.Time      `json:"generatedAt"`
	SpareVersion string         `json:"spareVersion"`
	Platform     Platform       `json:"platform"`
	Daemon       Daemon         `json:"daemon"`
	Machine      *Machine       `json:"machine,omitempty"`
	Jobs         []Job          `json:"jobs"`
	Packages     []Package      `json:"packages"`
	Diagnostics  []Diagnostic   `json:"diagnostics"`
	State        StateInventory `json:"state"`
}

type Platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type Daemon struct {
	Reachable bool `json:"reachable"`
}

type Machine struct {
	LogicalCores          int                `json:"logicalCores"`
	MemoryTotalBytes      uint64             `json:"memoryTotalBytes"`
	StorageAvailableBytes uint64             `json:"storageAvailableBytes"`
	Capabilities          model.Capabilities `json:"capabilities"`
}

type Job struct {
	RecipeID     string `json:"recipeId"`
	Mode         string `json:"mode"`
	DesiredState string `json:"desiredState"`
	Status       string `json:"status"`
	ProblemCode  string `json:"problemCode,omitempty"`
}

type Package struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	Publisher       string `json:"publisher"`
	SignatureStatus string `json:"signatureStatus"`
}

type Diagnostic struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type StateInventory struct {
	DatabasePresent     bool `json:"databasePresent"`
	EndpointPresent     bool `json:"endpointPresent"`
	InstallStatePresent bool `json:"installStatePresent"`
}

func Create(ctx context.Context, destination, spareVersion string, client *api.Client, statePaths paths.Paths) (string, error) {
	destination, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if info, statErr := os.Lstat(destination); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("the support bundle destination cannot be a symlink")
		}
		return "", errors.New("the support bundle destination already exists")
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}

	value := collect(ctx, spareVersion, client, statePaths)
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	temporary, err := os.CreateTemp(filepath.Dir(destination), ".spare-support-*")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", err
	}
	archive := zip.NewWriter(temporary)
	if err := writeEntry(archive, "README.txt", []byte(readme)); err != nil {
		_ = archive.Close()
		_ = temporary.Close()
		return "", err
	}
	if err := writeEntry(archive, "support.json", data); err != nil {
		_ = archive.Close()
		_ = temporary.Close()
		return "", err
	}
	if err := archive.Close(); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Link(temporaryPath, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func collect(ctx context.Context, spareVersion string, client *api.Client, statePaths paths.Paths) Bundle {
	diagnosticContext, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ctx = diagnosticContext
	value := Bundle{
		Schema:       Schema,
		GeneratedAt:  time.Now().UTC(),
		SpareVersion: spareVersion,
		Platform:     Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH},
		Jobs:         []Job{},
		Packages:     []Package{},
		State: StateInventory{
			DatabasePresent:     regularFileExists(statePaths.Database),
			EndpointPresent:     regularFileExists(statePaths.Endpoint),
			InstallStatePresent: regularFileExists(statePaths.InstallState),
		},
	}
	for _, report := range []doctor.Report{doctor.Run(ctx, client, statePaths), doctor.RunSecurity(ctx, client, statePaths)} {
		for _, check := range report.Checks {
			value.Diagnostics = append(value.Diagnostics, Diagnostic{ID: check.ID, Status: check.Status})
		}
	}
	if client == nil || client.Health(ctx) != nil {
		return value
	}
	value.Daemon.Reachable = true
	if machine, err := client.Machine(ctx); err == nil {
		value.Machine = &Machine{
			LogicalCores:          machine.LogicalCores,
			MemoryTotalBytes:      machine.MemoryTotalBytes,
			StorageAvailableBytes: machine.StorageAvailableBytes,
			Capabilities:          machine.Capabilities,
		}
	}
	if instances, err := client.Instances(ctx); err == nil {
		for _, instance := range instances {
			job := Job{RecipeID: instance.RecipeID, Mode: instance.Mode, DesiredState: instance.DesiredState, Status: instance.Status}
			if instance.Problem != nil {
				job.ProblemCode = instance.Problem.Code
			}
			value.Jobs = append(value.Jobs, job)
		}
	}
	if packages, err := client.JobPackages(ctx); err == nil {
		for _, item := range packages {
			value.Packages = append(value.Packages, Package{ID: item.ID, Version: item.Version, Publisher: item.Publisher, SignatureStatus: item.SignatureStatus})
		}
	}
	sort.Slice(value.Jobs, func(i, j int) bool { return value.Jobs[i].RecipeID < value.Jobs[j].RecipeID })
	sort.Slice(value.Packages, func(i, j int) bool { return value.Packages[i].ID < value.Packages[j].ID })
	sort.Slice(value.Diagnostics, func(i, j int) bool { return value.Diagnostics[i].ID < value.Diagnostics[j].ID })
	return value
}

func regularFileExists(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func writeEntry(archive *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	header.SetModTime(time.Unix(0, 0).UTC())
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}

const readme = `Spare support bundle

This bundle intentionally excludes API tokens, hostnames, machine IDs, IP
addresses, URLs, file paths, configuration values, logs, activity contents,
backups, job data, and files from user-selected folders.
`

func DefaultName(now time.Time) string {
	return fmt.Sprintf("spare-support-%s.zip", now.UTC().Format("20060102T150405Z"))
}
