package recipe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/spare-run/spare/internal/artifacts"
	"github.com/spare-run/spare/internal/config"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/permissions"
	"gopkg.in/yaml.v3"
)

const SchemaV1 = "spare.recipe/v1"

var recipeID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Manifest struct {
	Schema       string                  `json:"schema" yaml:"schema"`
	SpareVersion int                     `json:"-" yaml:"spare,omitempty"`
	ID           string                  `json:"id" yaml:"id"`
	Name         string                  `json:"name" yaml:"name"`
	Version      string                  `json:"version" yaml:"version"`
	Description  string                  `json:"description" yaml:"description"`
	Support      SupportSpec             `json:"support" yaml:"support"`
	Runtime      RuntimeSpec             `json:"runtime" yaml:"runtime"`
	Resources    ResourceSpec            `json:"resources" yaml:"resources"`
	Network      NetworkSpec             `json:"network" yaml:"network"`
	Storage      StorageSpec             `json:"storage" yaml:"storage"`
	Health       HealthSpec              `json:"health" yaml:"health"`
	Config       map[string]config.Field `json:"config" yaml:"config"`
	Permissions  permissions.Set         `json:"permissions" yaml:"permissions"`
}

type SupportSpec struct {
	Systems       []string `json:"systems" yaml:"systems"`
	Architectures []string `json:"architectures" yaml:"architectures"`
}

type RuntimeSpec struct {
	Type      string            `json:"type" yaml:"type"`
	Command   string            `json:"command,omitempty" yaml:"command,omitempty"`
	Arguments []string          `json:"arguments,omitempty" yaml:"arguments,omitempty"`
	Artifacts map[string]string `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
}

type ResourceSpec struct {
	MemoryRecommendedBytes uint64 `json:"memoryRecommendedBytes" yaml:"memoryRecommendedBytes"`
	MemoryMaximumBytes     uint64 `json:"memoryMaximumBytes" yaml:"memoryMaximumBytes"`
	CPUMaximum             int    `json:"cpuMaximum" yaml:"cpuMaximum"`
	StorageMinimumBytes    uint64 `json:"storageMinimumBytes" yaml:"storageMinimumBytes"`
}

type NetworkSpec struct {
	Visibility string `json:"visibility" yaml:"visibility"`
	Port       string `json:"port" yaml:"port"`
}

type StorageSpec struct {
	PathField string `json:"pathField" yaml:"pathField"`
	ReadOnly  bool   `json:"readOnly" yaml:"readOnly"`
}

type HealthSpec struct {
	Type             string `json:"type" yaml:"type"`
	Path             string `json:"path" yaml:"path"`
	IntervalSeconds  int    `json:"intervalSeconds" yaml:"intervalSeconds"`
	FailureThreshold int    `json:"failureThreshold" yaml:"failureThreshold"`
}

func Parse(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse recipe manifest: %w", err)
	}
	if manifest.Schema == "" && manifest.SpareVersion == 1 {
		manifest.Schema = SchemaV1
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Load(source string) (Manifest, error) {
	info, err := os.Stat(source)
	if err != nil {
		return Manifest{}, err
	}
	var data []byte
	if info.IsDir() {
		for _, name := range []string{"spare.yml", "recipe.yml"} {
			data, err = os.ReadFile(filepath.Join(source, name))
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return Manifest{}, err
			}
		}
	} else if strings.EqualFold(filepath.Ext(source), ".sp") {
		for _, name := range []string{"spare.yml", "recipe.yml"} {
			data, err = artifacts.ReadFile(source, name)
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return Manifest{}, err
			}
		}
	} else {
		data, err = os.ReadFile(source)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, errors.New("recipe must contain spare.yml")
		}
		return Manifest{}, err
	}
	return Parse(data)
}

func Pack(source, destination string) (Manifest, error) {
	manifest, err := Load(source)
	if err != nil {
		return Manifest{}, err
	}
	if destination == "" {
		destination = manifest.ID + ".sp"
	}
	if strings.EqualFold(filepath.Ext(destination), ".sp") == false {
		return Manifest{}, errors.New("recipe package output must end in .sp")
	}
	if err := artifacts.PackDirectory(source, destination); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Validate(manifest Manifest) error {
	if manifest.Schema != SchemaV1 {
		return fmt.Errorf("unsupported recipe schema %q; use %s", manifest.Schema, SchemaV1)
	}
	if !recipeID.MatchString(manifest.ID) {
		return errors.New("recipe id must start with a lowercase letter and contain only lowercase letters, numbers, or dashes")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return errors.New("recipe name is required")
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return errors.New("recipe version is required")
	}
	if strings.TrimSpace(manifest.Description) == "" {
		return errors.New("recipe description is required")
	}
	if manifest.Runtime.Type != "native" && manifest.Runtime.Type != "process" {
		return errors.New("runtime type must be native or process")
	}
	if len(manifest.Support.Systems) == 0 || len(manifest.Support.Architectures) == 0 {
		return errors.New("recipe support must declare systems and architectures")
	}
	if manifest.Network.Visibility != "" && manifest.Network.Visibility != "local" {
		return errors.New("V1 recipes may use only local network visibility")
	}
	if manifest.Storage.PathField != "" {
		field, ok := manifest.Config[manifest.Storage.PathField]
		if !ok || field.Type != config.TypeDirectory {
			return fmt.Errorf("storage pathField %q must reference a directory configuration field", manifest.Storage.PathField)
		}
	}
	for id, field := range manifest.Config {
		if !recipeID.MatchString(id) {
			return fmt.Errorf("invalid configuration field id %q", id)
		}
		switch field.Type {
		case config.TypeString, config.TypeDirectory, config.TypeSize, config.TypeBoolean, config.TypeInteger:
		default:
			return fmt.Errorf("configuration field %q has unsupported type %q", id, field.Type)
		}
		if strings.TrimSpace(field.Label) == "" {
			return fmt.Errorf("configuration field %q needs a label", id)
		}
	}
	return nil
}

func (manifest Manifest) Compatible(machine model.Machine) model.Compatibility {
	result := model.Compatibility{Supported: true, Rating: "Excellent"}
	if !contains(manifest.Support.Systems, machine.OS) {
		result.Supported = false
		result.Rating = "Unsupported"
		result.Reasons = append(result.Reasons, "This recipe does not support "+machine.OS+".")
	}
	if !contains(manifest.Support.Architectures, machine.Architecture) {
		result.Supported = false
		result.Rating = "Unsupported"
		result.Reasons = append(result.Reasons, "This recipe does not support "+machine.Architecture+".")
	}
	if manifest.Resources.MemoryRecommendedBytes > 0 {
		if machine.MemoryTotalBytes >= manifest.Resources.MemoryRecommendedBytes {
			result.Reasons = append(result.Reasons, "This computer has enough memory.")
		} else {
			result.Rating = "Limited"
			result.Warnings = append(result.Warnings, "This computer has less than the recommended memory.")
		}
	}
	if manifest.Resources.StorageMinimumBytes > 0 {
		if machine.StorageAvailableBytes >= manifest.Resources.StorageMinimumBytes {
			result.Reasons = append(result.Reasons, "This computer has enough available storage.")
		} else {
			result.Supported = false
			result.Rating = "Unsupported"
			result.Reasons = append(result.Reasons, "This computer does not have enough available storage.")
		}
	}
	if machine.Capabilities.HasBattery {
		result.Warnings = append(result.Warnings, "This computer may sleep when its lid is closed.")
	}
	if manifest.Network.Visibility == "local" && !machine.Capabilities.CanServeLAN {
		result.Rating = "Limited"
		result.Warnings = append(result.Warnings, "No local network address is currently available.")
	}
	if len(result.Reasons) == 0 && result.Supported {
		result.Reasons = append(result.Reasons, "This system and architecture are supported.")
	}
	return result
}

func (manifest Manifest) Model(machine model.Machine) model.Recipe {
	fields := make([]model.ConfigField, 0, len(manifest.Config))
	ids := make([]string, 0, len(manifest.Config))
	for id := range manifest.Config {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		field := manifest.Config[id]
		fields = append(fields, model.ConfigField{
			ID:          id,
			Type:        field.Type,
			Label:       field.Label,
			Description: field.Description,
			Required:    field.Required,
			Default:     field.Default,
		})
	}
	statements := permissions.Describe(manifest.Permissions)
	grants := make([]model.PermissionGrant, 0, len(statements))
	for _, statement := range statements {
		grants = append(grants, model.PermissionGrant{
			ID:          statement.ID,
			Description: statement.Description,
			Granted:     statement.Granted,
		})
	}
	return model.Recipe{
		ID:               manifest.ID,
		Title:            manifest.Name,
		Version:          manifest.Version,
		Description:      manifest.Description,
		Runtime:          manifest.Runtime.Type,
		SupportedSystems: append([]string(nil), manifest.Support.Systems...),
		Resources: model.ResourceGuidance{
			MemoryRecommendedBytes: manifest.Resources.MemoryRecommendedBytes,
			MemoryMaximumBytes:     manifest.Resources.MemoryMaximumBytes,
			CPUMaximum:             manifest.Resources.CPUMaximum,
		},
		Config:        fields,
		Permissions:   grants,
		Compatibility: manifest.Compatible(machine),
	}
}

func CurrentPlatformCompatible(manifest Manifest) model.Compatibility {
	return manifest.Compatible(model.Machine{
		OS:                    runtime.GOOS,
		Architecture:          runtime.GOARCH,
		MemoryTotalBytes:      ^uint64(0),
		StorageAvailableBytes: ^uint64(0),
		Capabilities:          model.Capabilities{CanServeLAN: true, CanRunPersistent: true},
	})
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
