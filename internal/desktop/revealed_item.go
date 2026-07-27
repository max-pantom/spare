package desktop

import (
	"errors"
	"os"
	"path/filepath"
)

func resolveRevealedItem(root, name string) (string, error) {
	if root == "" || name == "" || filepath.Base(name) != name ||
		name == "." || name == string(filepath.Separator) {
		return "", errors.New("choose a file from the active Drop")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", errors.New("Drop's selected folder is unavailable")
	}
	candidate := filepath.Join(resolvedRoot, name)
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", errors.New("the received file is no longer available")
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil || relative == ".." ||
		(len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator)) {
		return "", errors.New("the received file is outside Drop's selected folder")
	}
	info, err := os.Stat(resolvedCandidate)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("the received item is not a regular file")
	}
	return resolvedCandidate, nil
}
