//go:build desktop

package desktop

import (
	"testing"

	"github.com/spare-run/spare/internal/model"
)

func TestPresentTrayWithoutJob(t *testing.T) {
	presentation := presentTray(Snapshot{})
	if presentation.Headline != "No job" ||
		presentation.OpenLabel != "Choose a job" ||
		presentation.HasInstance ||
		presentation.IconState != trayIconNeutral {
		t.Fatalf("unexpected empty presentation: %#v", presentation)
	}
}

func TestPresentTrayReadyDrop(t *testing.T) {
	snapshot := trayTestSnapshot()
	presentation := presentTray(snapshot)
	if presentation.Headline != "Drop" ||
		presentation.Status != "Ready" ||
		presentation.OpenLabel != "Open received files" ||
		presentation.ToggleLabel != "Pause Drop" ||
		presentation.Address != "http://192.168.1.24:7340" ||
		presentation.IconState != trayIconReady {
		t.Fatalf("unexpected ready presentation: %#v", presentation)
	}
}

func TestPresentTrayActiveTransfer(t *testing.T) {
	snapshot := trayTestSnapshot()
	snapshot.Events = []model.Event{{
		InstanceID: "drop",
		Kind:       "drop_file_receiving",
		Details: map[string]any{
			"itemName": "campaign.zip",
			"progress": 62,
		},
	}}
	presentation := presentTray(snapshot)
	if presentation.Headline != "Receiving campaign.zip" ||
		presentation.Status != "62%" ||
		presentation.IconState != trayIconWorking {
		t.Fatalf("unexpected transfer presentation: %#v", presentation)
	}
}

func TestPresentTrayStorageFailure(t *testing.T) {
	snapshot := trayTestSnapshot()
	snapshot.Instances[0].Status = model.StatusDegraded
	snapshot.Instances[0].Problem = &model.Problem{
		Code:    "selected_folder_unavailable",
		Summary: "Storage folder is unavailable.",
	}
	presentation := presentTray(snapshot)
	if presentation.Headline != "Drop needs attention" ||
		presentation.Status != "Storage folder is unavailable" ||
		!presentation.NeedsAttention ||
		!presentation.CanReconnect ||
		presentation.IconState != trayIconWarning {
		t.Fatalf("unexpected failure presentation: %#v", presentation)
	}
}

func trayTestSnapshot() Snapshot {
	return Snapshot{
		Recipes: []model.Recipe{{
			ID:    model.RecipeDrop,
			Title: "Drop",
		}},
		Instances: []model.Instance{{
			ID:           "drop",
			RecipeID:     model.RecipeDrop,
			Status:       model.StatusHealthy,
			DesiredState: model.DesiredRunning,
			DataPath:     "/Users/max/Downloads/Spare",
			URLs: []string{
				"http://127.0.0.1:7340",
				"http://192.168.1.24:7340",
			},
		}},
	}
}
