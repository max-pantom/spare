package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Snapshot struct {
	Status                string `json:"status"`
	StorageAvailableBytes uint64 `json:"storageAvailableBytes,omitempty"`
	ItemCount             int    `json:"itemCount,omitempty"`
}

type Provider func() Snapshot

func Start(port int, provider Provider) (*http.Server, error) {
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet && request.Method != http.MethodHead {
				response.Header().Set("Allow", "GET, HEAD")
				response.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			response.Header().Set("Content-Type", "application/json")
			response.Header().Set("Cache-Control", "no-store")
			snapshot := Snapshot{Status: "healthy"}
			if provider != nil {
				snapshot = provider()
			}
			if snapshot.Status == "" {
				snapshot.Status = "healthy"
			}
			_ = json.NewEncoder(response).Encode(snapshot)
		}),
	}
	go func() {
		_ = server.Serve(listener)
	}()
	return server, nil
}

type Checker struct {
	Client *http.Client
}

func (c Checker) Check(ctx context.Context, port int) (Snapshot, error) {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
	if err != nil {
		return Snapshot{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return Snapshot{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Snapshot{}, fmt.Errorf("health endpoint returned HTTP %d", response.StatusCode)
	}
	var snapshot Snapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Status != "healthy" {
		return snapshot, fmt.Errorf("worker reported %s", snapshot.Status)
	}
	return snapshot, nil
}
