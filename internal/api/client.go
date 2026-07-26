package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

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
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > 4096 {
		return "", errors.New("the API token file is invalid")
	}
	return string(data), nil
}

func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/v1/health", nil, nil)
}

func (c *Client) Machine(ctx context.Context) (model.Machine, error) {
	var result model.Machine
	err := c.do(ctx, http.MethodGet, "/api/v1/machine", nil, &result)
	return result, err
}

func (c *Client) Instances(ctx context.Context) ([]model.Instance, error) {
	var result []model.Instance
	err := c.do(ctx, http.MethodGet, "/api/v1/instances", nil, &result)
	return result, err
}

func (c *Client) Create(ctx context.Context, mode, rootPath, portMode string, port int) (model.Instance, error) {
	body := map[string]any{
		"recipeId": model.RecipeSite,
		"mode":     mode,
		"config": map[string]any{
			"rootPath": rootPath,
			"portMode": portMode,
			"port":     port,
		},
	}
	var result model.Instance
	err := c.do(ctx, http.MethodPost, "/api/v1/instances", body, &result)
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
