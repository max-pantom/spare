package pairing

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPairAuthorizePersistAndRevoke(t *testing.T) {
	root := t.TempDir()
	gate, err := New("123456", "Clipboard", root)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(gate.Middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("paired"))
	})))
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	request, err := http.NewRequest(http.MethodPost, server.URL+"/pair", strings.NewReader(url.Values{
		"device": {"My phone"},
		"code":   {"123456"},
	}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", server.URL)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("pair status = %d", response.StatusCode)
	}

	response, err = client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authorized status = %d", response.StatusCode)
	}
	pairedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	connectedAt := gate.now().UTC()
	gate.now = func() time.Time { return connectedAt.Add(lastSeenInterval) }
	foreignRequest := httptest.NewRequest(http.MethodGet, server.URL+"/", nil)
	foreignRequest.RemoteAddr = "192.0.2.25:4321"
	for _, cookie := range jar.Cookies(pairedURL) {
		foreignRequest.AddCookie(cookie)
	}
	if !gate.Authorized(foreignRequest) {
		t.Fatal("paired session was rejected after the device address changed")
	}
	devices, err := ReadConnectedDevices(root, gate.now())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("connected devices = %d", len(devices))
	}
	if devices[0].Name != "My phone" {
		t.Fatalf("device name = %q", devices[0].Name)
	}
	if !devices[0].LastSeen.Equal(gate.now().UTC()) {
		t.Fatalf("last seen = %s, want %s", devices[0].LastSeen, gate.now().UTC())
	}
	info, err := os.Stat(filepath.Join(root, "paired-devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("paired device mode = %o", info.Mode().Perm())
	}

	request, err = http.NewRequest(http.MethodPost, server.URL+"/pair/revoke", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", server.URL)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.Request.URL.Path != "/pair" {
		t.Fatalf("revoke ended at %s", response.Request.URL.Path)
	}
}

func TestPairRejectsForeignOrigin(t *testing.T) {
	gate, err := New("123456", "Monitor", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(gate.Middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})))
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/pair", strings.NewReader("code=123456"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://attacker.example")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestPairAcceptsEquivalentLoopbackOrigin(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		origin      string
		remote      string
		wantAllowed bool
	}{
		{
			name:        "localhost origin for ipv4 loopback",
			host:        "127.0.0.1:7340",
			origin:      "http://localhost:7340",
			remote:      "192.0.2.12:54000",
			wantAllowed: true,
		},
		{
			name:        "ipv4 origin for localhost",
			host:        "localhost:7340",
			origin:      "http://127.0.0.1:7340",
			remote:      "192.0.2.12:54000",
			wantAllowed: true,
		},
		{
			name:        "different port",
			host:        "localhost:7340",
			origin:      "http://127.0.0.1:7341",
			remote:      "192.0.2.12:54000",
			wantAllowed: false,
		},
		{
			name:        "foreign host",
			host:        "localhost:7340",
			origin:      "http://attacker.example:7340",
			remote:      "192.0.2.12:54000",
			wantAllowed: false,
		},
		{
			name:        "Wails application origin on this computer",
			host:        "127.0.0.1:7340",
			origin:      "wails://wails",
			remote:      "127.0.0.1:54000",
			wantAllowed: true,
		},
		{
			name:        "sandboxed application origin on this computer",
			host:        "127.0.0.1:7340",
			origin:      "null",
			remote:      "127.0.0.1:54000",
			wantAllowed: true,
		},
		{
			name:        "Wails origin from a nearby device",
			host:        "192.0.2.20:7340",
			origin:      "wails://wails",
			remote:      "192.0.2.12:54000",
			wantAllowed: false,
		},
		{
			name:        "sandboxed origin from a nearby device",
			host:        "192.0.2.20:7340",
			origin:      "null",
			remote:      "192.0.2.12:54000",
			wantAllowed: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://"+test.host+"/pair", nil)
			request.Host = test.host
			request.RemoteAddr = test.remote
			request.Header.Set("Origin", test.origin)
			if allowed := validOrigin(request); allowed != test.wantAllowed {
				t.Fatalf("validOrigin() = %t, want %t", allowed, test.wantAllowed)
			}
		})
	}
}

func TestLocalJobAccessDoesNotRequirePairing(t *testing.T) {
	gate, err := New("123456", "Clipboard", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := gate.Middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7340/", nil)
	request.RemoteAddr = "127.0.0.1:54000"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("local status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "http://192.0.2.20:7340/", nil)
	request.RemoteAddr = "192.0.2.12:54000"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("nearby status = %d", response.Code)
	}
}

func TestPairRateLimitsRepeatedWrongCodes(t *testing.T) {
	gate, err := New("123456", "Downloads", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(gate.Middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})))
	defer server.Close()
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for index := 0; index < maxPairFailures; index++ {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/pair", strings.NewReader("code=000000"))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", server.URL)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusSeeOther {
			t.Fatalf("attempt %d status = %d", index+1, response.StatusCode)
		}
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/pair", strings.NewReader("code=123456"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", server.URL)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d", response.StatusCode)
	}
}

func TestPairBoundsPersistedSessions(t *testing.T) {
	gate, err := New("123456", "Monitor", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < maxPairSessions; index++ {
		gate.sessions[fmt.Sprintf("%064x", index)] = session{
			Device:    "Device",
			Address:   "192.0.2.10",
			ExpiresAt: time.Now().UTC().Add(time.Duration(index+1) * time.Minute),
		}
	}
	gate.makeSessionRoomLocked()
	if len(gate.sessions) != maxPairSessions-1 {
		t.Fatalf("session count = %d", len(gate.sessions))
	}
}

func TestPairRejectsOversizedState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "paired-devices.json"),
		bytes.Repeat([]byte("x"), maxPairStateSize+1),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := New("123456", "Monitor", root); err == nil {
		t.Fatal("expected oversized paired-device state to be rejected")
	}
}
