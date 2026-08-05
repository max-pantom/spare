package artifacts

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxPackageFileSize = 2 << 30
	maxPackageFiles    = 10_000
)

type PackageFile struct {
	Name           string `json:"name"`
	Size           uint64 `json:"size"`
	CompressedSize uint64 `json:"compressedSize"`
}

func PackDirectory(source, destination string) error {
	root, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("recipe source must be a directory")
	}
	var paths []string
	err = filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("recipe packages cannot contain symlinks: %s", current)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("recipe packages can contain only regular files: %s", current)
		}
		paths = append(paths, current)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".spare-package-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	archive := zip.NewWriter(temporary)
	for _, current := range paths {
		relative, err := filepath.Rel(root, current)
		if err != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return err
		}
		fileInfo, err := os.Stat(current)
		if err != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return err
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return err
		}
		header.Name = filepath.ToSlash(relative)
		header.Method = zip.Deflate
		header.SetModTime(time.Unix(0, 0).UTC())
		header.SetMode(fileInfo.Mode().Perm())
		writer, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return err
		}
		file, err := os.Open(current)
		if err != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return copyErr
		}
		if closeErr != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return closeErr
		}
	}
	if err := archive.Close(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return atomicReplace(temporaryPath, destination)
}

func ReadFile(packagePath, name string) ([]byte, error) {
	return ReadFileLimit(packagePath, name, maxPackageFileSize)
}

func ReadFileLimit(packagePath, name string, maximum int64) ([]byte, error) {
	if maximum <= 0 || maximum > maxPackageFileSize {
		return nil, errors.New("invalid package file size limit")
	}
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	cleanName, err := cleanArchivePath(name)
	if err != nil {
		return nil, err
	}
	for _, file := range archive.File {
		if file.Name != cleanName {
			continue
		}
		if file.UncompressedSize64 > uint64(maximum) {
			return nil, errors.New("package file is too large")
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, maximum+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if int64(len(data)) > maximum {
			return nil, errors.New("package file is too large")
		}
		return data, nil
	}
	return nil, os.ErrNotExist
}

func ListFiles(packagePath string) ([]PackageFile, error) {
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	if len(archive.File) > maxPackageFiles {
		return nil, fmt.Errorf("package contains more than %d files", maxPackageFiles)
	}
	files := make([]PackageFile, 0, len(archive.File))
	seen := make(map[string]string, len(archive.File))
	for _, file := range archive.File {
		cleanName, err := cleanArchivePath(file.Name)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(cleanName)
		if existing, ok := seen[key]; ok {
			return nil, fmt.Errorf("package contains duplicate or case-conflicting paths: %s and %s", existing, cleanName)
		}
		seen[key] = cleanName
		mode := file.FileInfo().Mode()
		if mode&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("package contains a symlink: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if !mode.IsRegular() {
			return nil, fmt.Errorf("package contains a special file: %s", file.Name)
		}
		if file.UncompressedSize64 > maxPackageFileSize {
			return nil, fmt.Errorf("package file is too large: %s", file.Name)
		}
		files = append(files, PackageFile{
			Name:           cleanName,
			Size:           file.UncompressedSize64,
			CompressedSize: file.CompressedSize64,
		})
	}
	sort.Slice(files, func(left, right int) bool {
		return files[left].Name < files[right].Name
	})
	return files, nil
}

func Extract(packagePath, destination string) error {
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	root, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	for _, file := range archive.File {
		cleanName, err := cleanArchivePath(file.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(cleanName))
		if !within(root, target) {
			return fmt.Errorf("package path escapes its destination: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("package contains a symlink: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if file.UncompressedSize64 > maxPackageFileSize {
			return fmt.Errorf("package file is too large: %s", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.Mode().Perm())
		if err != nil {
			_ = reader.Close()
			return err
		}
		written, copyErr := io.Copy(output, io.LimitReader(reader, maxPackageFileSize+1))
		closeOutputErr := output.Close()
		closeReaderErr := reader.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		if closeReaderErr != nil {
			return closeReaderErr
		}
		if written > maxPackageFileSize {
			return fmt.Errorf("package file is too large: %s", file.Name)
		}
	}
	return nil
}

func cleanArchivePath(value string) (string, error) {
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "./")
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "/") ||
		cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid package path %q", value)
	}
	return cleaned, nil
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
