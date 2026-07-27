//go:build desktop

package desktop

import (
	"context"
	"time"

	"github.com/spare-run/spare/internal/model"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startBackgroundLoops() {
	go a.keepTemporaryLease()
	go a.streamActivity()
}

func (a *App) keepTemporaryLease() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			client, err := a.ensureConnected(a.ctx)
			if err != nil {
				continue
			}
			instances, err := client.Instances(a.ctx)
			if err != nil {
				continue
			}
			for _, instance := range instances {
				if instance.Mode == model.ModeTemporary {
					_ = client.Heartbeat(a.ctx, instance.ID)
				}
			}
		}
	}
}

func (a *App) streamActivity() {
	for {
		if a.ctx.Err() != nil {
			return
		}
		client, err := a.ensureConnected(a.ctx)
		if err != nil {
			if !waitContext(a.ctx, 2*time.Second) {
				return
			}
			continue
		}
		err = client.StreamActivity(a.ctx, func(event model.Event) {
			wailsruntime.EventsEmit(a.ctx, "spare:activity", event)
			a.notify(event)
			_, _ = a.Snapshot()
		})
		if err != nil && !waitContext(a.ctx, time.Second) {
			return
		}
	}
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
