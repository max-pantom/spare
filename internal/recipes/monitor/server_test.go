package monitor

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type monitorRoundTripFunc func(*http.Request) (*http.Response, error)

func (function monitorRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestMonitorChecksHTTPAndTCP(t *testing.T) {
	web := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer web.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	server, err := newServer(map[string]any{
		"pairing-code":   "123456",
		"check-interval": int64(30),
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []target{
		{Type: "http", Address: web.URL},
		{Type: "tcp", Address: listener.Addr().String()},
	} {
		result := server.runCheck(candidate)
		if result.Status != "up" {
			t.Fatalf("%s result = %#v", candidate.Type, result)
		}
	}
}

func TestMonitorTargetValidationAndDegradedHealth(t *testing.T) {
	for _, value := range []struct {
		kind    string
		address string
	}{
		{"http", "file:///tmp/value"},
		{"tcp", "missing-port"},
		{"ping", "-c"},
		{"other", "example.com"},
	} {
		if _, err := validateTarget(value.kind, value.address); err == nil {
			t.Fatalf("expected %#v to fail", value)
		}
	}
	server, err := newServer(map[string]any{
		"pairing-code":   "123456",
		"check-interval": int64(30),
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server.targets = []target{{Status: "down"}, {Status: "up"}}
	snapshot := server.health()
	if snapshot.Status != "degraded" || snapshot.ProblemCode != "monitor_target_down" {
		t.Fatalf("health = %#v", snapshot)
	}
}

func TestMonitorBoundsTargetsAndDoesNotExposeURLSecrets(t *testing.T) {
	server, err := newServer(map[string]any{
		"pairing-code":   "123456",
		"check-interval": int64(30),
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server.targets = make([]target, maxMonitorTargets)
	form := url.Values{
		"name":    {"One more"},
		"type":    {"http"},
		"address": {"https://example.com/health"},
	}
	request := httptest.NewRequest(http.MethodPost, "/targets", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.add(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("target limit status = %d", response.Code)
	}

	const secretAddress = "https://example.com/health?token=do-not-display"
	server.client = &http.Client{Transport: monitorRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, errors.New("request failed for " + request.URL.String())
	})}
	result := server.runCheck(target{Type: "http", Address: secretAddress})
	if strings.Contains(result.Message, "do-not-display") ||
		strings.Contains(result.Message, secretAddress) {
		t.Fatalf("HTTP error exposed the target URL: %q", result.Message)
	}
	displayed := displayMonitorAddress(secretAddress)
	if strings.Contains(displayed, "do-not-display") {
		t.Fatalf("display address exposed query credentials: %q", displayed)
	}
}

func TestMonitorRejectsUnsafeRedirects(t *testing.T) {
	client := newMonitorHTTPClient()
	secure, err := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	insecure, err := http.NewRequest(http.MethodGet, "http://example.com/end", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(insecure, []*http.Request{secure}); err == nil {
		t.Fatal("expected an HTTPS downgrade redirect to be rejected")
	}
	withCredentials, err := http.NewRequest(http.MethodGet, "https://user:password@example.com/end", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(withCredentials, []*http.Request{secure}); err == nil {
		t.Fatal("expected a credential-bearing redirect to be rejected")
	}
}
