package joblibrary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/artifacts"
	"github.com/spare-run/spare/internal/jobpackage"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/state"
	"golang.org/x/mod/semver"
)

type Library struct {
	store          *state.Store
	root           string
	currentVersion string
	trusted        *recipe.Registry
	verifier       *jobpackage.Verifier
	bundled        map[string]struct{}
}

const (
	maxJobPackageBytes             = 16 * 1024 * 1024
	maxJobPackageFiles             = 32
	maxJobPackageFileBytes         = 8 * 1024 * 1024
	maxJobPackageUncompressedBytes = 24 * 1024 * 1024
)

func New(
	store *state.Store,
	root,
	currentVersion string,
	trusted *recipe.Registry,
	verifier *jobpackage.Verifier,
) (*Library, error) {
	if store == nil || trusted == nil || verifier == nil {
		return nil, errors.New("job library dependencies are required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, err
	}
	return &Library{
		store:          store,
		root:           root,
		currentVersion: currentVersion,
		trusted:        trusted,
		verifier:       verifier,
		bundled: map[string]struct{}{
			model.RecipeSite: {},
			model.RecipeDrop: {},
			model.RecipeHook: {},
		},
	}, nil
}

func (l *Library) Available(id string) bool {
	if _, ok := l.bundled[id]; ok {
		return true
	}
	_, err := l.validatedPackage(context.Background(), id)
	return err == nil
}

func (l *Library) Review(source string) (model.JobPackageReview, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return model.JobPackageReview{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return model.JobPackageReview{}, err
	}
	if !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(absolute), ".sp") {
		return model.JobPackageReview{}, errors.New("choose a regular .sp job package")
	}
	if err := validateJobPackageShape(absolute, info.Size()); err != nil {
		return model.JobPackageReview{}, err
	}
	verification, err := l.verifier.Verify(absolute)
	if err != nil {
		return model.JobPackageReview{}, err
	}
	manifest, err := recipe.Load(absolute)
	if err != nil {
		return model.JobPackageReview{}, err
	}
	if _, bundled := l.bundled[manifest.ID]; bundled {
		return model.JobPackageReview{}, errors.New("bundled jobs cannot be installed from the optional catalog")
	}
	implementation, ok := l.trusted.Get(manifest.ID)
	if !ok {
		return model.JobPackageReview{}, fmt.Errorf("%s needs a newer version of Spare", manifest.Name)
	}
	if !reflect.DeepEqual(manifest, implementation.Manifest()) {
		return model.JobPackageReview{}, fmt.Errorf("%s does not match the trusted implementation in this Spare release", manifest.Name)
	}
	if !compatibleVersion(l.currentVersion, verification.MinimumSpare) {
		return model.JobPackageReview{}, fmt.Errorf(
			"%s requires Spare %s or newer",
			manifest.Name,
			verification.MinimumSpare,
		)
	}
	checksum, err := artifacts.SHA256(absolute)
	if err != nil {
		return model.JobPackageReview{}, err
	}
	_, installedErr := l.store.JobPackage(context.Background(), manifest.ID)
	statements := permissions.Describe(manifest.Permissions)
	grants := make([]model.PermissionGrant, 0, len(statements))
	for _, statement := range statements {
		grants = append(grants, model.PermissionGrant{
			ID:          statement.ID,
			Description: statement.Description,
			Granted:     statement.Granted,
		})
	}
	return model.JobPackageReview{
		ID:               manifest.ID,
		Title:            manifest.Name,
		Version:          manifest.Version,
		Description:      manifest.Description,
		Publisher:        verification.Publisher,
		MinimumSpare:     verification.MinimumSpare,
		Checksum:         checksum,
		SignatureStatus:  "verified",
		Permissions:      grants,
		AlreadyInstalled: installedErr == nil,
	}, nil
}

func (l *Library) Install(source string) (model.JobPackage, error) {
	originalSource := source
	staged, err := l.stagePackage(source)
	if err != nil {
		return model.JobPackage{}, err
	}
	defer os.Remove(staged)

	review, err := l.Review(staged)
	if err != nil {
		return model.JobPackage{}, err
	}
	if current, currentErr := l.store.JobPackage(context.Background(), review.ID); currentErr == nil {
		if compareVersion(review.Version, current.Version) < 0 {
			return model.JobPackage{}, fmt.Errorf(
				"cannot downgrade %s from %s to %s",
				review.Title,
				current.Version,
				review.Version,
			)
		}
	}
	verification, err := l.verifier.Verify(staged)
	if err != nil {
		return model.JobPackage{}, err
	}
	manifest, err := recipe.Load(staged)
	if err != nil {
		return model.JobPackage{}, err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return model.JobPackage{}, err
	}
	name, err := jobPackageName(review.ID, review.Version)
	if err != nil {
		return model.JobPackage{}, err
	}
	destination := filepath.Join(l.root, name)
	if err := copyAtomic(staged, destination); err != nil {
		return model.JobPackage{}, err
	}
	installedReview, err := l.Review(destination)
	if err != nil ||
		installedReview.ID != review.ID ||
		installedReview.Version != review.Version ||
		!strings.EqualFold(installedReview.Checksum, review.Checksum) {
		_ = os.Remove(destination)
		if err != nil {
			return model.JobPackage{}, fmt.Errorf("verify installed job package: %w", err)
		}
		return model.JobPackage{}, errors.New("installed job package changed while it was being copied")
	}
	installedVerification, err := l.verifier.Verify(destination)
	if err != nil ||
		installedVerification.Publisher != verification.Publisher ||
		installedVerification.KeyID != verification.KeyID ||
		installedVerification.MinimumSpare != verification.MinimumSpare ||
		installedVerification.Digest != verification.Digest ||
		installedVerification.Signature != verification.Signature {
		_ = os.Remove(destination)
		if err != nil {
			return model.JobPackage{}, fmt.Errorf("verify installed job package signature: %w", err)
		}
		return model.JobPackage{}, errors.New("installed job package signature changed while it was being copied")
	}
	if _, err := recipe.Load(destination); err != nil {
		_ = os.Remove(destination)
		return model.JobPackage{}, err
	}
	value := model.JobPackage{
		ID:              review.ID,
		Version:         review.Version,
		Publisher:       review.Publisher,
		MinimumSpare:    review.MinimumSpare,
		Checksum:        review.Checksum,
		Signature:       verification.Signature,
		SignatureStatus: "verified",
		ManifestJSON:    manifestJSON,
		PackagePath:     destination,
		Source:          filepath.Base(originalSource),
		InstalledAt:     time.Now().UTC(),
	}
	if err := l.store.PutJobPackage(context.Background(), value); err != nil {
		_ = os.Remove(destination)
		return model.JobPackage{}, err
	}
	return value, nil
}

func validateJobPackageShape(path string, compressedSize int64) error {
	if compressedSize < 0 || compressedSize > maxJobPackageBytes {
		return fmt.Errorf("job package exceeds the %d-byte download limit", maxJobPackageBytes)
	}
	files, err := artifacts.ListFiles(path)
	if err != nil {
		return err
	}
	if len(files) > maxJobPackageFiles {
		return fmt.Errorf("job package contains more than %d files", maxJobPackageFiles)
	}
	allowed := map[string]bool{
		"README.md":              true,
		"icon.svg":               true,
		"recipe.yml":             true,
		"spare.yml":              true,
		jobpackage.SignatureFile: true,
	}
	manifestCount := 0
	var total uint64
	for _, file := range files {
		if !allowed[file.Name] {
			return fmt.Errorf("job package contains unexpected file %q", file.Name)
		}
		if file.Name == "spare.yml" || file.Name == "recipe.yml" {
			manifestCount++
		}
		if file.Size > maxJobPackageFileBytes {
			return fmt.Errorf("job package file %q exceeds the %d-byte limit", file.Name, maxJobPackageFileBytes)
		}
		if total > maxJobPackageUncompressedBytes-file.Size {
			return fmt.Errorf("job package expands beyond the %d-byte limit", maxJobPackageUncompressedBytes)
		}
		total += file.Size
	}
	if manifestCount != 1 {
		return errors.New("job package must contain exactly one manifest")
	}
	return nil
}

func (l *Library) stagePackage(source string) (string, error) {
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	temporary, err := os.CreateTemp(l.root, ".spare-review-*.sp")
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	remove := true
	defer func() {
		_ = temporary.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return "", err
	}
	written, err := io.Copy(temporary, io.LimitReader(input, maxJobPackageBytes+1))
	if err != nil {
		return "", err
	}
	if written > maxJobPackageBytes {
		return "", fmt.Errorf("job package exceeds the %d-byte download limit", maxJobPackageBytes)
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	remove = false
	return path, nil
}

func jobPackageName(id, version string) (string, error) {
	name := id + "_" + version + ".sp"
	if filepath.Base(name) != name ||
		strings.ContainsAny(name, `/\`) ||
		name == "." || name == ".." {
		return "", errors.New("job package has an unsafe identity or version")
	}
	return name, nil
}

func (l *Library) Packages(ctx context.Context) ([]model.JobPackage, error) {
	packages, err := l.store.JobPackages(ctx)
	if err != nil {
		return nil, err
	}
	for index := range packages {
		if _, validateErr := l.validatedPackage(ctx, packages[index].ID); validateErr != nil {
			packages[index].SignatureStatus = "invalid"
		}
	}
	return packages, nil
}

func (l *Library) Package(ctx context.Context, id string) (model.JobPackage, error) {
	return l.store.JobPackage(ctx, id)
}

func (l *Library) Uninstall(ctx context.Context, id string) error {
	if _, bundled := l.bundled[id]; bundled {
		return errors.New("bundled jobs cannot be uninstalled")
	}
	value, err := l.store.JobPackage(ctx, id)
	if err != nil {
		return err
	}
	if err := l.store.DeleteJobPackage(ctx, id); err != nil {
		return err
	}
	if err := os.Remove(value.PackagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = l.store.PutJobPackage(ctx, value)
		return err
	}
	return nil
}

func (l *Library) Decorate(recipes []model.Recipe) []model.Recipe {
	result := make([]model.Recipe, 0, len(recipes))
	for _, available := range recipes {
		if _, bundled := l.bundled[available.ID]; bundled {
			available.Installation = model.InstallationBundled
			available.Publisher = jobpackage.DefaultPublisher
			available.SignatureStatus = "bundled"
			result = append(result, available)
			continue
		}
		packaged, err := l.validatedPackage(context.Background(), available.ID)
		if err != nil {
			continue
		}
		available.Installation = model.InstallationInstalled
		available.Publisher = packaged.Publisher
		available.PackageVersion = packaged.Version
		available.MinimumSpare = packaged.MinimumSpare
		available.Checksum = packaged.Checksum
		available.SignatureStatus = packaged.SignatureStatus
		result = append(result, available)
	}
	return result
}

func (l *Library) validatedPackage(ctx context.Context, id string) (model.JobPackage, error) {
	packaged, err := l.store.JobPackage(ctx, id)
	if err != nil {
		return model.JobPackage{}, err
	}
	absoluteRoot, err := filepath.Abs(l.root)
	if err != nil {
		return model.JobPackage{}, err
	}
	absolutePackage, err := filepath.Abs(packaged.PackagePath)
	if err != nil {
		return model.JobPackage{}, err
	}
	relative, err := filepath.Rel(absoluteRoot, absolutePackage)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return model.JobPackage{}, errors.New("installed package is outside the private job library")
	}
	info, err := os.Stat(absolutePackage)
	if err != nil {
		return model.JobPackage{}, err
	}
	if !info.Mode().IsRegular() {
		return model.JobPackage{}, errors.New("installed package is not a regular file")
	}
	if err := validateJobPackageShape(absolutePackage, info.Size()); err != nil {
		return model.JobPackage{}, err
	}
	checksum, err := artifacts.SHA256(absolutePackage)
	if err != nil {
		return model.JobPackage{}, err
	}
	if !strings.EqualFold(checksum, packaged.Checksum) {
		return model.JobPackage{}, errors.New("installed package checksum has changed")
	}
	verification, err := l.verifier.Verify(absolutePackage)
	if err != nil {
		return model.JobPackage{}, err
	}
	if verification.Publisher != packaged.Publisher ||
		verification.MinimumSpare != packaged.MinimumSpare ||
		verification.Signature != packaged.Signature {
		return model.JobPackage{}, errors.New("installed package signature metadata has changed")
	}
	if !compatibleVersion(l.currentVersion, verification.MinimumSpare) {
		return model.JobPackage{}, errors.New("installed package is not compatible with this Spare release")
	}
	manifest, err := recipe.Load(absolutePackage)
	if err != nil {
		return model.JobPackage{}, err
	}
	if manifest.ID != packaged.ID || manifest.Version != packaged.Version {
		return model.JobPackage{}, errors.New("installed package identity has changed")
	}
	implementation, ok := l.trusted.Get(manifest.ID)
	if !ok || !reflect.DeepEqual(manifest, implementation.Manifest()) {
		return model.JobPackage{}, errors.New("installed package does not match a trusted implementation")
	}
	return packaged, nil
}

func compatibleVersion(current, minimum string) bool {
	if current == "" || current == "dev" {
		return true
	}
	return compareVersion(current, minimum) >= 0
}

func compareVersion(left, right string) int {
	left = normalizeVersion(left)
	right = normalizeVersion(right)
	if !semver.IsValid(left) || !semver.IsValid(right) {
		return strings.Compare(left, right)
	}
	return semver.Compare(left, right)
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	return value
}

func copyAtomic(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".spare-job-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := io.Copy(temporary, input); err != nil {
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
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(temporaryPath, destination)
}
