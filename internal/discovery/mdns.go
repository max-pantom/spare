package discovery

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/grandcat/zeroconf"
)

type serverCloser struct {
	server *zeroconf.Server
}

func (c serverCloser) Close() error {
	c.server.Shutdown()
	return nil
}

func Advertise(jobTitle, hostname, machineID string, port int) (io.Closer, error) {
	name := ServiceName(jobTitle, hostname, machineID)
	server, err := zeroconf.Register(name, "_http._tcp", "local.", port, []string{"path=/"}, nil)
	if err != nil {
		return nil, err
	}
	return serverCloser{server: server}, nil
}

func ServiceName(jobTitle, hostname, machineID string) string {
	jobTitle = strings.TrimSpace(jobTitle)
	if jobTitle == "" {
		jobTitle = "Job"
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "computer"
	}
	digest := sha256.Sum256([]byte(machineID))
	suffix := fmt.Sprintf(" [%x]", digest[:4])
	prefix := "Spare " + jobTitle + " on "
	if len(prefix)+len(suffix) >= 63 {
		prefix = "Spare on "
	}
	maximumHostnameBytes := 63 - len(prefix) - len(suffix)
	for len(hostname) > maximumHostnameBytes {
		_, size := utf8.DecodeLastRuneInString(hostname)
		hostname = hostname[:len(hostname)-size]
	}
	return prefix + hostname + suffix
}
