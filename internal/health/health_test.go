package health

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServerAndChecker(t *testing.T) {
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()
	server, err := Start(port, func() Snapshot {
		return Snapshot{Status: "healthy", ItemCount: 3}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var snapshot Snapshot
	for {
		snapshot, err = (Checker{}).Check(ctx, port)
		if err == nil || ctx.Err() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ItemCount != 3 {
		t.Fatalf("item count = %d", snapshot.ItemCount)
	}
}
