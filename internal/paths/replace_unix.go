//go:build !windows

package paths

import "os"

func atomicReplace(source, destination string) error {
	return os.Rename(source, destination)
}
