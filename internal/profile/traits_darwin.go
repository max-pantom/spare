//go:build darwin

package profile

import (
	"os"
	"os/exec"
	"strings"
)

func portableTraits() (bool, bool) {
	output, _ := exec.Command("pmset", "-g", "batt").Output()
	hasBattery := strings.Contains(string(output), "InternalBattery")
	entries, _ := os.ReadDir("/Volumes")
	hasExternal := false
	for _, entry := range entries {
		if entry.Name() != "Macintosh HD" && entry.Name() != "Recovery" {
			hasExternal = true
			break
		}
	}
	return hasBattery, hasExternal
}
