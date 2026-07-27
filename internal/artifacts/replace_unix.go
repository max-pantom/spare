//go:build !windows

package artifacts

import "os"

func atomicReplace(source, destination string) error {
	return os.Rename(source, destination)
}
