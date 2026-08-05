package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Recovery struct {
	DatabasePath string
}

// OpenRecovering preserves a structurally corrupt database and its SQLite
// sidecars before creating a fresh store. Permission, disk, and migration
// errors are returned without moving anything.
func OpenRecovering(path string) (*Store, *Recovery, error) {
	store, err := Open(path)
	if err == nil {
		return store, nil, nil
	}
	if !isSQLiteCorruption(err) {
		return nil, nil, err
	}

	recoveryPath, quarantineErr := quarantineDatabase(path, time.Now().UTC())
	if quarantineErr != nil {
		return nil, nil, fmt.Errorf("preserve corrupt Spare database: %w", quarantineErr)
	}
	store, openErr := Open(path)
	if openErr != nil {
		return nil, nil, fmt.Errorf("create fresh Spare database after preserving %s: %w", recoveryPath, openErr)
	}
	return store, &Recovery{DatabasePath: recoveryPath}, nil
}

func quarantineDatabase(path string, now time.Time) (string, error) {
	suffix := ".corrupt-" + now.Format("20060102T150405.000000000Z")
	recoveryPath := path + suffix
	type move struct{ source, destination string }
	var moves []move
	for _, source := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Lstat(source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not a regular file", source)
		}
		destination := recoveryPath + source[len(path):]
		if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return "", fmt.Errorf("recovery destination already exists: %s", destination)
			}
			return "", err
		}
		if err := os.Chmod(source, 0o600); err != nil {
			return "", err
		}
		moves = append(moves, move{source: source, destination: destination})
	}
	for _, candidate := range moves {
		if err := os.Rename(candidate.source, candidate.destination); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(recoveryPath); err != nil {
		return "", fmt.Errorf("corrupt database was not preserved: %w", err)
	}
	return filepath.Clean(recoveryPath), nil
}
