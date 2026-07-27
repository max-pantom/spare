package artifacts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const maxArtifactSize = 2 << 30

func Download(ctx context.Context, client *http.Client, url, destination string) error {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download %s: HTTP %d", url, response.StatusCode)
	}
	if response.ContentLength > maxArtifactSize {
		return fmt.Errorf("artifact exceeds the %d-byte limit", maxArtifactSize)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".spare-download-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	written, copyErr := io.Copy(temporary, io.LimitReader(response.Body, maxArtifactSize+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maxArtifactSize {
		return fmt.Errorf("artifact exceeds the %d-byte limit", maxArtifactSize)
	}
	return atomicReplace(temporaryPath, destination)
}
