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
