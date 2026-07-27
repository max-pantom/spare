package uninstall

import (
	"context"
	"os"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/service"
)

// Remove stops every recipe, unregisters the per-user daemon, and removes
// Spare state. Selected recipe folders are never removed.
func Remove(ctx context.Context, statePaths paths.Paths) error {
	var endpoint paths.Endpoint
	if client, err := api.Discover(statePaths); err == nil {
		endpoint, _ = statePaths.ReadEndpoint()
		if instances, listErr := client.Instances(ctx); listErr == nil {
			for _, current := range instances {
				_ = client.Remove(ctx, current.ID)
			}
		}
	}
	if err := service.Uninstall(ctx, statePaths.Root); err != nil {
		return err
	}
	if os.Getenv("SPARE_NO_SERVICE") == "1" && endpoint.PID > 0 {
		if process, err := os.FindProcess(endpoint.PID); err == nil {
			_ = process.Kill()
		}
	}
	time.Sleep(200 * time.Millisecond)
	return os.RemoveAll(statePaths.Root)
}
