package profile

import (
	"testing"
	"time"

	"github.com/spare-run/spare/internal/model"
)

func TestCollectPreservesMachineIdentity(t *testing.T) {
	initialized := time.Now().Add(-time.Hour).UTC()
	existing := &model.Machine{
		ID:            "spare_existing",
		InitializedAt: initialized,
	}
	machine, err := Collect(existing, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if machine.ID != existing.ID {
		t.Fatalf("machine ID changed: %q", machine.ID)
	}
	if !machine.InitializedAt.Equal(initialized) {
		t.Fatalf("initialized time changed: %s", machine.InitializedAt)
	}
	if machine.Hostname == "" || machine.OS == "" || machine.Architecture == "" {
		t.Fatalf("profile is incomplete: %#v", machine)
	}
}
