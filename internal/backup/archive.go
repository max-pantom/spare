package backup

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Export(instanceRoot string, manifest Manifest, destination string) error {
	root, err := filepath.Abs(instanceRoot)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("backup source must be a folder")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".spare-backup-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	archive := zip.NewWriter(temporary)
	manifestWriter, err := archive.Create("backup.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(manifestWriter).Encode(manifest); err != nil {
		return err
	}
	var files []string
	err = filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup cannot include symlink %s", current)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("backup can include only regular files: %s", current)
		}
		files = append(files, current)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, current := range files {
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		writer, err := archive.Create("data/" + filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		source, err := os.Open(current)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destination)
}

func Inspect(source string) (Manifest, error) {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return Manifest{}, err
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != "backup.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return Manifest{}, err
		}
		var manifest Manifest
		decodeErr := json.NewDecoder(io.LimitReader(reader, 1024*1024)).Decode(&manifest)
		closeErr := reader.Close()
		if decodeErr != nil {
			return Manifest{}, decodeErr
		}
		if closeErr != nil {
			return Manifest{}, closeErr
		}
		if manifest.Schema != SchemaV1 || manifest.RecipeID == "" {
			return Manifest{}, errors.New("unsupported or invalid Spare backup")
		}
		return manifest, nil
	}
	return Manifest{}, errors.New("backup.json is missing")
}

func Import(source, destination string) (Manifest, error) {
	manifest, err := Inspect(source)
	if err != nil {
		return Manifest{}, err
	}
	entries, err := os.ReadDir(destination)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(destination, 0o700); err != nil {
			return Manifest{}, err
		}
	} else if err != nil {
		return Manifest{}, err
	} else if len(entries) != 0 {
		return Manifest{}, errors.New("backup destination must be empty")
	}
	root, err := filepath.Abs(destination)
	if err != nil {
		return Manifest{}, err
	}
	archive, err := zip.OpenReader(source)
	if err != nil {
		return Manifest{}, err
	}
	defer archive.Close()
	for _, file := range archive.File {
		if !strings.HasPrefix(file.Name, "data/") || file.Name == "data/" {
			continue
		}
		relative := strings.TrimPrefix(file.Name, "data/")
		cleaned := filepath.Clean(filepath.FromSlash(relative))
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || filepath.IsAbs(cleaned) {
			return Manifest{}, fmt.Errorf("backup contains invalid path %q", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return Manifest{}, fmt.Errorf("backup contains symlink %q", file.Name)
		}
		target := filepath.Join(root, cleaned)
		relativeTarget, err := filepath.Rel(root, target)
		if err != nil || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
			return Manifest{}, fmt.Errorf("backup path escapes its destination: %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return Manifest{}, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return Manifest{}, err
		}
		reader, err := file.Open()
		if err != nil {
			return Manifest{}, err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = reader.Close()
			return Manifest{}, err
		}
		_, copyErr := io.Copy(output, reader)
		closeOutputErr := output.Close()
		closeReaderErr := reader.Close()
		if copyErr != nil {
			return Manifest{}, copyErr
		}
		if closeOutputErr != nil {
			return Manifest{}, closeOutputErr
		}
		if closeReaderErr != nil {
			return Manifest{}, closeReaderErr
		}
	}
	return manifest, nil
}
