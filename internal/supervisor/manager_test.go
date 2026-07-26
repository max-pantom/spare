package supervisor

import (
	"net"
	"testing"
)

func TestSelectPortHonorsFixedCollision(t *testing.T) {
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	if _, err := selectPort(port, "fixed"); err == nil {
		t.Fatal("expected a fixed port collision")
	}
}

func TestMDNSHostname(t *testing.T) {
	if actual := mdnsHostname("Max’s Mac.local"); actual != "max-s-mac" {
		t.Fatalf("unexpected hostname: %q", actual)
	}
}
