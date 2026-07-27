//go:build linux

package profile

import (
	"os"
	"path/filepath"
)

func portableTraits() (bool, bool) {
	batteries, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	return len(batteries) > 0, hasMountedEntries("/media") || hasMountedEntries("/mnt")
}

func hasMountedEntries(root string) bool {
	entries, err := os.ReadDir(root)
	return err == nil && len(entries) > 0
}
