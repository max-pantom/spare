package doctor

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spare-run/spare/internal/auth"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
)

func TestStorageChecksDoNotRemoveSelectedFolder(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	checks := storageChecks(model.Instance{ID: "drop", RecipeID: "drop", DataPath: root})
	if len(checks) < 2 {
		t.Fatalf("checks = %#v", checks)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("doctor changed the selected folder")
	}
}

func TestPrivateStateCheckDetectsAndRepairsBroadTokenPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permissions are enforced with ACLs rather than Unix modes")
	}

	statePaths := paths.FromRoot(t.TempDir())
	if err := statePaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.EnsureToken(statePaths.Token); err != nil {
		t.Fatal(err)
	}
	if check := privateStateCheck(statePaths); check.Status != "healthy" {
		t.Fatalf("initial check = %#v", check)
	}

	if err := os.Chmod(statePaths.Token, 0o644); err != nil {
		t.Fatal(err)
	}
	if check := privateStateCheck(statePaths); check.Status != "failed" {
		t.Fatalf("broad permission check = %#v", check)
	}

	if _, err := auth.EnsureToken(statePaths.Token); err != nil {
		t.Fatal(err)
	}
	if check := privateStateCheck(statePaths); check.Status != "healthy" {
		t.Fatalf("repaired check = %#v", check)
	}
}

func TestWorkerIsolationCheckIsExplicit(t *testing.T) {
	check := workerIsolationCheck()
	if check.Status != "healthy" && (check.Status != "warning" || check.Recovery == "") {
		t.Fatalf("check = %#v", check)
	}
}

func TestMissingSelectedFolderHasRecoveryInstructions(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "disconnected drive", "Drop")
	checks := storageChecks(model.Instance{ID: "drop", RecipeID: model.RecipeDrop, DataPath: missing})
	if len(checks) != 1 || checks[0].Status != "failed" || checks[0].Recovery == "" {
		t.Fatalf("missing-folder checks = %#v", checks)
	}
}

func TestNetworkChecksExplainUnverifiedFirewallAccess(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	checks := networkChecks(model.Instance{ID: "drop", Port: port, URLs: []string{
		"http://127.0.0.1:7340",
		"http://192.168.1.20:7340",
	}})
	for _, check := range checks {
		if check.ID == "firewall.drop" {
			if check.Status != "warning" || check.Recovery == "" {
				t.Fatalf("firewall check = %#v", check)
			}
			return
		}
	}
	t.Fatal("firewall check was not reported")
}
