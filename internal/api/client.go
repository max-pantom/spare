package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/auth"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type ClientError struct {
	Status int
	API    model.APIError
}

func (e *ClientError) Error() string {
	if e.API.Hint != "" {
		return e.API.Message + " " + e.API.Hint
	}
	return e.API.Message
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func Discover(paths paths.Paths) (*Client, error) {
	endpoint, err := paths.ReadEndpoint()
	if err != nil {
		return nil, fmt.Errorf("Spare is not running: %w", err)
	}
	token, err := ReadToken(paths.Token)
	if err != nil {
		return nil, err
	}
	return NewClient(endpoint.URL, token), nil
}

func ReadToken(path string) (string, error) {
	return auth.ReadToken(path)
}

func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/v1/health", nil, nil)
}

func (c *Client) Machine(ctx context.Context) (model.Machine, error) {
	var result model.Machine
	err := c.do(ctx, http.MethodGet, "/api/v1/machine", nil, &result)
	return result, err
}

func (c *Client) Recipes(ctx context.Context) ([]model.Recipe, error) {
	var result []model.Recipe
	err := c.do(ctx, http.MethodGet, "/api/v1/recipes", nil, &result)
	return result, err
}

func (c *Client) JobPackages(ctx context.Context) ([]model.JobPackage, error) {
	var result []model.JobPackage
	err := c.do(ctx, http.MethodGet, "/api/v1/job-packages", nil, &result)
	return result, err
}

func (c *Client) ReviewJobPackage(ctx context.Context, source string) (model.JobPackageReview, error) {
	var result model.JobPackageReview
	err := c.do(ctx, http.MethodPost, "/api/v1/job-packages/review", map[string]string{
		"source": source,
	}, &result)
	return result, err
}

func (c *Client) InstallJobPackage(ctx context.Context, source string) (model.JobPackage, error) {
	var result model.JobPackage
	err := c.do(ctx, http.MethodPost, "/api/v1/job-packages/install", map[string]string{
		"source": source,
	}, &result)
	return result, err
}

func (c *Client) UninstallJobPackage(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/job-packages/"+id, nil, nil)
}

func (c *Client) JobProfile(ctx context.Context, id string) (model.JobProfile, error) {
	var result model.JobProfile
	err := c.do(ctx, http.MethodGet, "/api/v1/job-profiles/"+id, nil, &result)
	return result, err
}

func (c *Client) Instances(ctx context.Context) ([]model.Instance, error) {
	var result []model.Instance
	err := c.do(ctx, http.MethodGet, "/api/v1/instances", nil, &result)
	return result, err
}

func (c *Client) Instance(ctx context.Context, id string) (model.Instance, error) {
	var result model.Instance
	err := c.do(ctx, http.MethodGet, "/api/v1/instances/"+id, nil, &result)
	return result, err
}

func (c *Client) Create(
	ctx context.Context,
	recipeID,
	mode string,
	config map[string]any,
	portMode string,
	port int,
) (model.Instance, error) {
	body := map[string]any{
		"recipeId": recipeID,
		"mode":     mode,
		"config":   config,
		"portMode": portMode,
		"port":     port,
	}
	var result model.Instance
	err := c.do(ctx, http.MethodPost, "/api/v1/instances", body, &result)
	return result, err
}

func (c *Client) Switch(
	ctx context.Context,
	recipeID,
	mode string,
	config map[string]any,
	portMode string,
	port int,
) (model.Instance, error) {
	body := map[string]any{
		"recipeId": recipeID,
		"mode":     mode,
		"config":   config,
		"portMode": portMode,
		"port":     port,
	}
	var result model.Instance
	err := c.do(ctx, http.MethodPost, "/api/v1/instances/switch", body, &result)
	return result, err
}

func (c *Client) InstanceAction(ctx context.Context, id, action string) (model.Instance, error) {
	var result model.Instance
	err := c.do(ctx, http.MethodPost, "/api/v1/instances/"+id+"/"+action, nil, &result)
	return result, err
}

func (c *Client) Heartbeat(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/api/v1/instances/"+id+"/heartbeat", nil, nil)
}

func (c *Client) Promote(ctx context.Context, id string) (model.Instance, error) {
	var result model.Instance
	err := c.do(ctx, http.MethodPost, "/api/v1/instances/"+id+"/promote", nil, &result)
	return result, err
}

func (c *Client) Configure(
	ctx context.Context,
	id,
	recipeID string,
	config map[string]any,
	portMode string,
	port int,
) (model.Instance, error) {
	var result model.Instance
	err := c.do(ctx, http.MethodPost, "/api/v1/instances/"+id+"/configure", map[string]any{
		"recipeId": recipeID,
		"config":   config,
		"portMode": portMode,
		"port":     port,
	}, &result)
	return result, err
}

func (c *Client) Remove(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/instances/"+id, nil, nil)
}

func (c *Client) Events(ctx context.Context, limit int) ([]model.Event, error) {
	var result []model.Event
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/events?limit=%d", limit), nil, &result)
	return result, err
}

func (c *Client) BrowserSession(ctx context.Context) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	err := c.do(ctx, http.MethodPost, "/api/v1/browser-sessions", nil, &result)
	return result.URL, err
}

func (c *Client) ExportBackup(ctx context.Context, instanceID, destination string) error {
	return c.do(ctx, http.MethodPost, "/api/v1/desktop/backups/export", map[string]any{
		"instanceId":  instanceID,
		"destination": destination,
	}, nil)
}

func (c *Client) RestoreBackup(ctx context.Context, source, destination string) (model.Instance, error) {
	var result model.Instance
	err := c.do(ctx, http.MethodPost, "/api/v1/desktop/backups/restore", map[string]any{
		"source":      source,
		"destination": destination,
	}, &result)
	return result, err
}

func (c *Client) AddDropFiles(ctx context.Context, instanceID string, paths []string) ([]string, error) {
	var result struct {
		Names []string `json:"names"`
	}
	err := c.do(ctx, http.MethodPost, "/api/v1/desktop/drop-files", map[string]any{
		"instanceId": instanceID,
		"paths":      paths,
	}, &result)
	return result.Names, err
}

// StreamActivity receives committed events until ctx is cancelled. Callers
// should reconnect and refresh /events when this stream returns.
func (c *Client) StreamActivity(ctx context.Context, receive func(model.Event)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/activity/stream", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "text/event-stream")
	// Activity is a long-lived response. The bounded client used for ordinary
	// API calls would otherwise tear down a healthy stream every ten seconds.
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Spare returned HTTP %d", response.StatusCode)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event model.Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err == nil {
			receive(event)
		}
	}
	return scanner.Err()
}

func (c *Client) do(ctx context.Context, method, route string, body any, output any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+route, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var envelope model.ErrorEnvelope
		if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			return fmt.Errorf("Spare returned HTTP %d", response.StatusCode)
		}
		return &ClientError{Status: response.StatusCode, API: envelope.Error}
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(output)
}
