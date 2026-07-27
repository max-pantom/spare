package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type DroppedPath struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func describeDroppedPaths(paths []string) ([]DroppedPath, error) {
	if len(paths) > 100 {
		return nil, errors.New("drop no more than 100 items at once")
	}
	result := make([]DroppedPath, 0, len(paths))
	for _, source := range paths {
		absolute, err := filepath.Abs(source)
		if err != nil {
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, err
		}
		kind := "file"
		switch {
		case info.IsDir():
			kind = "directory"
		case !info.Mode().IsRegular():
			kind = "unsupported"
		case strings.EqualFold(filepath.Ext(resolved), ".sp"):
			kind = "recipe-package"
		case strings.EqualFold(filepath.Ext(resolved), ".spare-backup"):
			kind = "backup"
		}
		result = append(result, DroppedPath{
			Path: resolved,
			Name: filepath.Base(resolved),
			Kind: kind,
		})
	}
	return result, nil
}
