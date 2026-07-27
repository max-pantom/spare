package drop

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/profile"
)

var errInvalidFilename = errors.New("the uploaded file needs a valid name")

type fileEntry struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
	URL        string    `json:"url"`
}

func listFiles(root string) ([]fileEntry, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	result := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		result = append(result, fileEntry{
			Name:       entry.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC(),
			URL:        "/files/" + pathEscape(entry.Name()),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ModifiedAt.After(result[j].ModifiedAt)
	})
	return result, nil
}

func downloadPath(root, name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return "", os.ErrNotExist
	}
	candidate := filepath.Join(root, name)
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", os.ErrNotExist
	}
	return candidate, nil
}

func collisionFreePath(root, name string) (string, error) {
	name = safeFilename(name)
	if name == "" {
		return "", errInvalidFilename
	}
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	for index := 0; index < 10_000; index++ {
		candidateName := name
		if index > 0 {
			candidateName = base + " (" + integerString(index) + ")" + extension
		}
		candidate := filepath.Join(root, candidateName)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("unable to choose a free filename")
}

func safeFilename(value string) string {
	if value == "" || value != strings.TrimSpace(value) ||
		value == "." || value == ".." || strings.HasPrefix(value, ".") ||
		value != filepath.Base(value) || strings.ContainsAny(value, `/\<>:"|?*`) {
		return ""
	}
	for _, character := range value {
		if character < 32 || character == 127 {
			return ""
		}
	}
	return value
}

func availableStorage(root string) uint64 {
	return profile.StorageAvailable(root)
}
