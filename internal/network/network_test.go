package network

import (
	"net"
	"strings"
	"testing"
)

func TestSelectPort(t *testing.T) {
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	used := listener.Addr().(*net.TCPAddr).Port
	if _, err := SelectPort(used, "fixed"); err == nil {
		t.Fatal("expected fixed-port collision")
	}
	port, err := SelectPort(0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if port < FirstRecipePort || port > LastRecipePort {
		t.Fatalf("automatic port = %d", port)
	}
}

func TestAutomaticPortSkipsCollision(t *testing.T) {
	listener, err := net.Listen("tcp4", "0.0.0.0:7340")
	if err != nil {
		t.Skipf("first automatic port is already occupied outside the test: %v", err)
	}
	defer listener.Close()
	port, err := SelectPort(0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if port == FirstRecipePort {
		t.Fatalf("automatic selection reused occupied port %d", port)
	}
}

func TestEndpointsReflectAddressChanges(t *testing.T) {
	before := URLs(EndpointsForAddresses("Max's Mac", 7340, []string{"192.168.1.20"}))
	after := URLs(EndpointsForAddresses("Max's Mac", 7340, []string{"10.0.0.8"}))
	if strings.Join(before, "\n") == strings.Join(after, "\n") {
		t.Fatalf("endpoints did not change: %v", after)
	}
	if !strings.Contains(strings.Join(after, "\n"), "10.0.0.8:7340") ||
		strings.Contains(strings.Join(after, "\n"), "192.168.1.20") {
		t.Fatalf("refreshed endpoints = %v", after)
	}
}

func TestLocalHostname(t *testing.T) {
	if value := LocalHostname(" Max’s MacBook.local "); value != "max-s-macbook" {
		t.Fatalf("hostname = %q", value)
	}
	unicodeOnly := LocalHostname("東京のMac")
	if unicodeOnly == "" || len(unicodeOnly) > 63 {
		t.Fatalf("unicode hostname = %q", unicodeOnly)
	}
	if unicodeOnly != LocalHostname("東京のMac") {
		t.Fatal("unicode hostname fallback is not stable")
	}
	if value := LocalHostname(strings.Repeat("very-long-hostname-", 8)); len(value) > 63 {
		t.Fatalf("long hostname has %d bytes", len(value))
	}
}
