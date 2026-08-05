package network

import (
	"crypto/sha256"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spare-run/spare/internal/profile"
)

const (
	FirstRecipePort = 7340
	LastRecipePort  = 7399
)

type Error struct {
	Code    string
	Message string
	Hint    string
}

func (e *Error) Error() string {
	return e.Message
}

type Endpoint struct {
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

func SelectPort(requested int, mode string) (int, error) {
	if mode == "fixed" || requested > 0 {
		if requested < 1 || requested > 65535 {
			return 0, &Error{Code: "invalid_port", Message: "Choose a port between 1 and 65535."}
		}
		if !PortAvailable(requested) {
			return 0, &Error{
				Code:    "port_in_use",
				Message: fmt.Sprintf("Port %d is already in use.", requested),
				Hint:    "Use `--port auto` or choose another port.",
			}
		}
		return requested, nil
	}
	for port := FirstRecipePort; port <= LastRecipePort; port++ {
		if PortAvailable(port) {
			return port, nil
		}
	}
	return 0, &Error{
		Code:    "no_recipe_port_available",
		Message: "Spare could not find a free recipe port.",
		Hint:    "Close another local service or choose a specific free port.",
	}
}

func PortAvailable(port int) bool {
	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func FreeLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func Endpoints(hostname string, port int) []Endpoint {
	return EndpointsForAddresses(hostname, port, profile.LANAddresses())
}

func EndpointsForAddresses(hostname string, port int, addresses []string) []Endpoint {
	result := []Endpoint{{
		Kind:     "loopback",
		URL:      fmt.Sprintf("http://127.0.0.1:%d", port),
		Hostname: "127.0.0.1",
		Port:     port,
	}}
	for _, address := range addresses {
		result = append(result, Endpoint{
			Kind:     "lan",
			URL:      "http://" + net.JoinHostPort(address, strconv.Itoa(port)),
			Hostname: address,
			Port:     port,
		})
	}
	if localName := LocalHostname(hostname); localName != "" {
		result = append(result, Endpoint{
			Kind:     "mdns",
			URL:      fmt.Sprintf("http://%s.local:%d", localName, port),
			Hostname: localName + ".local",
			Port:     port,
		})
	}
	return result
}

func URLs(endpoints []Endpoint) []string {
	result := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		result = append(result, endpoint.URL)
	}
	return result
}

func LocalHostname(value string) string {
	original := strings.ToLower(strings.TrimSpace(value))
	value = original
	value = strings.TrimSuffix(value, ".local")
	var result strings.Builder
	lastDash := false
	for _, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if valid {
			result.WriteRune(character)
			lastDash = false
		} else if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	label := strings.Trim(result.String(), "-")
	if len(label) > 63 {
		label = strings.TrimRight(label[:63], "-")
	}
	if label == "" && original != "" {
		digest := sha256.Sum256([]byte(original))
		label = fmt.Sprintf("spare-%x", digest[:5])
	}
	return label
}
