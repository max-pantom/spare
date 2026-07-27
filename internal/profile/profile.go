package profile

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/spare-run/spare/internal/model"
)

func Collect(existing *model.Machine, storagePath string) (model.Machine, error) {
	now := time.Now().UTC()
	hostname, _ := os.Hostname()
	lanAddresses := LANAddresses()
	availableStorage := storageAvailable(storagePath)
	hasBattery, hasExternalStorage := portableTraits()
	id := ""
	initializedAt := now
	if existing != nil {
		id = existing.ID
		initializedAt = existing.InitializedAt
	}
	if id == "" {
		id = randomID()
	}

	return model.Machine{
		ID:                    id,
		Hostname:              hostname,
		OS:                    runtime.GOOS,
		Architecture:          runtime.GOARCH,
		LogicalCores:          runtime.NumCPU(),
		MemoryTotalBytes:      totalMemory(),
		StorageAvailableBytes: availableStorage,
		LANAddresses:          lanAddresses,
		Capabilities: model.Capabilities{
			CanServeLAN:        len(lanAddresses) > 0,
			CanRunPersistent:   true,
			CanStoreLargeFiles: availableStorage >= 10*1024*1024*1024,
			CanRunContainers:   runtime.GOOS == "linux",
			HasBattery:         hasBattery,
			HasExternalStorage: hasExternalStorage,
		},
		InitializedAt:  initializedAt,
		LastProfiledAt: now,
	}, nil
}

func LANAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var result []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			var ip net.IP
			switch value := address.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			text := ip.String()
			if !seen[text] {
				seen[text] = true
				result = append(result, text)
			}
		}
	}
	sort.Strings(result)
	return result
}

func StorageAvailable(path string) uint64 {
	return storageAvailable(path)
}

func randomID() string {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return "spare_" + hex.EncodeToString(value)
}
