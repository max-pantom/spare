package network

import (
	"net"
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
}
