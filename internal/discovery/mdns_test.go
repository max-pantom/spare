package discovery

import (
	"strings"
	"testing"
)

func TestServiceNameHandlesUnicodeAndAvoidsMachineConflicts(t *testing.T) {
	first := ServiceName("Drop", "東京のとても長いコンピューター名東京のとても長いコンピューター名", "machine-one")
	second := ServiceName("Drop", "東京のとても長いコンピューター名東京のとても長いコンピューター名", "machine-two")
	if first == second {
		t.Fatal("different machines received the same mDNS service name")
	}
	if len(first) > 63 || len(second) > 63 {
		t.Fatalf("service names exceed DNS-SD limit: %d, %d", len(first), len(second))
	}
	if first != ServiceName("Drop", "東京のとても長いコンピューター名東京のとても長いコンピューター名", "machine-one") {
		t.Fatal("service name is not stable")
	}
	if !strings.HasPrefix(first, "Spare Drop on ") {
		t.Fatalf("service name = %q", first)
	}
}
