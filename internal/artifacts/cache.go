package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Cache struct {
	Root string
}

func (c Cache) Path(source string) string {
	hash := sha256.Sum256([]byte(source))
	name := "artifact"
	if parsed, err := url.Parse(source); err == nil {
		if base := filepath.Base(parsed.Path); base != "." && base != "/" && base != "" {
			name = base
		}
	}
	return filepath.Join(c.Root, hex.EncodeToString(hash[:8])+"-"+name)
}

func (c Cache) CleanOlderThan(cutoff time.Time) error {
	entries, err := os.ReadDir(c.Root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(c.Root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
