package doctor

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/model"
)

func networkChecks(instance model.Instance) []Check {
	result := []Check{}
	address := fmt.Sprintf("127.0.0.1:%d", instance.Port)
	connection, err := net.DialTimeout("tcp4", address, 500*time.Millisecond)
	if err != nil {
		result = append(result, Check{
			ID:       "port." + instance.ID,
			Name:     "Recipe port",
			Status:   "failed",
			Message:  fmt.Sprintf("Port %d is not reachable on this computer.", instance.Port),
			Recovery: "Check the recipe log, then stop and start the recipe.",
		})
	} else {
		_ = connection.Close()
		result = append(result, Check{
			ID:      "port." + instance.ID,
			Name:    "Recipe port",
			Status:  "healthy",
			Message: fmt.Sprintf("Port %d is reachable on this computer.", instance.Port),
		})
	}
	mdns := false
	lan := false
	for _, value := range instance.URLs {
		mdns = mdns || strings.Contains(value, ".local:")
		lan = lan || (!strings.Contains(value, "127.0.0.1") && !strings.Contains(value, ".local:"))
	}
	if lan {
		result = append(result, Check{
			ID:      "lan." + instance.ID,
			Name:    "Local network",
			Status:  "ready",
			Message: "Spare found an address for nearby devices.",
		})
	} else {
		result = append(result, Check{
			ID:       "lan." + instance.ID,
			Name:     "Local network",
			Status:   "warning",
			Message:  "No LAN address is currently available.",
			Recovery: "Connect this computer to Wi-Fi or Ethernet and try again.",
		})
	}
	if mdns {
		result = append(result, Check{
			ID:      "mdns." + instance.ID,
			Name:    "Local name",
			Status:  "ready",
			Message: "A .local address is available for discovery.",
		})
	}
	return result
}
