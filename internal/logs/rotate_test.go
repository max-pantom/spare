package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingWriterKeepsBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "site.log")
	writer, err := NewRotatingWriter(path, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("12345678")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("ABCDEFGH")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{path, path + ".1", path + ".2"} {
		if _, err := os.Stat(expected); err != nil {
			t.Errorf("expected %s: %v", expected, err)
		}
	}
}
