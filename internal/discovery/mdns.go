package discovery

import (
	"io"
	"strings"

	"github.com/grandcat/zeroconf"
)

type serverCloser struct {
	server *zeroconf.Server
}

func (c serverCloser) Close() error {
	c.server.Shutdown()
	return nil
}

func Advertise(hostname string, port int) (io.Closer, error) {
	name := "Spare Site"
	if value := strings.TrimSpace(hostname); value != "" {
		name += " on " + value
	}
	server, err := zeroconf.Register(name, "_http._tcp", "local.", port, []string{"path=/"}, nil)
	if err != nil {
		return nil, err
	}
	return serverCloser{server: server}, nil
}
