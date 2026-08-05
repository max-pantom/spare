package pairing

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const cookieName = "spare_job_session"

const (
	maxPairFailures  = 5
	maxPairSessions  = 100
	maxPairStateSize = 1024 * 1024
	pairWindow       = 5 * time.Minute
	pairLockout      = 10 * time.Minute
	lastSeenInterval = 10 * time.Minute
)

type session struct {
	Device    string    `json:"device"`
	Address   string    `json:"address"`
	PairedAt  time.Time `json:"pairedAt,omitempty"`
	LastSeen  time.Time `json:"lastSeen,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ConnectedDevice is the safe, token-free view of a paired session that may be
// shown in the local desktop application.
type ConnectedDevice struct {
	Name      string    `json:"name"`
	PairedAt  time.Time `json:"pairedAt"`
	LastSeen  time.Time `json:"lastSeen"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type attempt struct {
	Failures      int
	WindowStarted time.Time
	LockedUntil   time.Time
}

type diskState struct {
	Sessions map[string]session `json:"sessions"`
}

type Gate struct {
	mu       sync.Mutex
	code     string
	title    string
	path     string
	sessions map[string]session
	attempts map[string]attempt
	now      func() time.Time
}

func New(code, title, dataPath string) (*Gate, error) {
	code = strings.TrimSpace(code)
	if len(code) < 6 {
		return nil, errors.New("the pairing code must contain at least 6 characters")
	}
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dataPath, 0o700); err != nil {
		return nil, err
	}
	gate := &Gate{
		code:     code,
		title:    title,
		path:     filepath.Join(dataPath, "paired-devices.json"),
		sessions: map[string]session{},
		attempts: map[string]attempt{},
		now:      time.Now,
	}
	if err := gate.load(); err != nil {
		return nil, err
	}
	return gate, nil
}

func GenerateCode() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	number := (uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])) % 1_000_000
	digits := []byte("000000")
	for index := len(digits) - 1; index >= 0; index-- {
		digits[index] = byte('0' + number%10)
		number /= 10
	}
	return string(digits), nil
}

func WithGeneratedCode(input map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(input)+1)
	for key, value := range input {
		result[key] = value
	}
	if value, ok := result["pairing-code"].(string); !ok || strings.TrimSpace(value) == "" {
		code, err := GenerateCode()
		if err != nil {
			return nil, err
		}
		result["pairing-code"] = code
	}
	return result, nil
}

func (g *Gate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/pair" {
			g.servePair(response, request)
			return
		}
		if !requestIsLoopback(request) && !g.Authorized(request) {
			http.Redirect(response, request, "/pair", http.StatusSeeOther)
			return
		}
		if request.Method == http.MethodPost && !validOrigin(request) {
			http.Error(response, "This request did not come from this Spare job.", http.StatusForbidden)
			return
		}
		if request.URL.Path == "/pair/revoke" {
			g.revoke(response, request)
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (g *Gate) Authorized(request *http.Request) bool {
	cookie, err := request.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	key := tokenHash(cookie.Value)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pruneLocked()
	value, ok := g.sessions[key]
	if !ok {
		return false
	}
	now := g.now().UTC()
	if !value.ExpiresAt.After(now) {
		return false
	}
	if value.LastSeen.IsZero() || now.Sub(value.LastSeen) >= lastSeenInterval {
		previous := value
		value.LastSeen = now
		value.Address = requestAddress(request)
		g.sessions[key] = value
		// A failed activity timestamp write must not invalidate an otherwise
		// valid session. The next request can retry the update.
		if err := g.saveLocked(); err != nil {
			g.sessions[key] = previous
		}
	}
	return true
}

func (g *Gate) servePair(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		setBrowserHeaders(response)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_ = pairTemplate.Execute(response, map[string]any{
			"Title": g.title,
			"Error": request.URL.Query().Get("error") == "code",
		})
	case http.MethodPost:
		if !validOrigin(request) {
			http.Error(response, "This request did not come from this Spare job.", http.StatusForbidden)
			return
		}
		if g.blocked(request) {
			response.Header().Set("Retry-After", "600")
			http.Error(response, "Too many pairing attempts. Wait ten minutes and try again.", http.StatusTooManyRequests)
			return
		}
		request.Body = http.MaxBytesReader(response, request.Body, 4096)
		if err := request.ParseForm(); err != nil {
			http.Error(response, "Unable to read the pairing form.", http.StatusBadRequest)
			return
		}
		submittedCode := strings.TrimSpace(request.FormValue("code"))
		if subtle.ConstantTimeCompare([]byte(submittedCode), []byte(g.code)) != 1 {
			g.recordFailure(request)
			http.Redirect(response, request, "/pair?error=code", http.StatusSeeOther)
			return
		}
		g.clearFailures(request)
		token, err := randomToken()
		if err != nil {
			http.Error(response, "Unable to pair this device.", http.StatusInternalServerError)
			return
		}
		device := strings.TrimSpace(request.FormValue("device"))
		if device == "" {
			device = "Nearby device"
		}
		deviceRunes := []rune(device)
		if len(deviceRunes) > 80 {
			device = string(deviceRunes[:80])
		}
		now := g.now().UTC()
		expires := now.Add(24 * time.Hour)
		g.mu.Lock()
		g.makeSessionRoomLocked()
		g.sessions[tokenHash(token)] = session{
			Device:    device,
			Address:   requestAddress(request),
			PairedAt:  now,
			LastSeen:  now,
			ExpiresAt: expires,
		}
		err = g.saveLocked()
		g.mu.Unlock()
		if err != nil {
			http.Error(response, "Unable to save this paired device.", http.StatusInternalServerError)
			return
		}
		http.SetCookie(response, &http.Cookie{
			Name:     cookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   request.TLS != nil,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   int((24 * time.Hour).Seconds()),
		})
		http.Redirect(response, request, "/", http.StatusSeeOther)
	default:
		response.Header().Set("Allow", "GET, POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (g *Gate) blocked(request *http.Request) bool {
	key := requestAddress(request)
	now := g.now().UTC()
	g.mu.Lock()
	defer g.mu.Unlock()
	value, ok := g.attempts[key]
	if !ok {
		return false
	}
	if value.LockedUntil.After(now) {
		return true
	}
	if now.Sub(value.WindowStarted) >= pairWindow {
		delete(g.attempts, key)
	}
	return false
}

func (g *Gate) recordFailure(request *http.Request) {
	key := requestAddress(request)
	now := g.now().UTC()
	g.mu.Lock()
	defer g.mu.Unlock()
	value := g.attempts[key]
	if value.WindowStarted.IsZero() || now.Sub(value.WindowStarted) >= pairWindow {
		value = attempt{WindowStarted: now}
	}
	value.Failures++
	if value.Failures >= maxPairFailures {
		value.LockedUntil = now.Add(pairLockout)
	}
	g.attempts[key] = value
}

func (g *Gate) clearFailures(request *http.Request) {
	g.mu.Lock()
	delete(g.attempts, requestAddress(request))
	g.mu.Unlock()
}

func requestAddress(request *http.Request) string {
	value := request.RemoteAddr
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	if address := net.ParseIP(value); address != nil {
		return address.String()
	}
	return value
}

// ReadConnectedDevices reads token-free pairing metadata for the desktop UI.
// Expired sessions and secret session hashes are never returned.
func ReadConnectedDevices(dataPath string, now time.Time) ([]ConnectedDevice, error) {
	path := filepath.Join(dataPath, "paired-devices.json")
	state, err := readDiskState(path)
	if errors.Is(err, os.ErrNotExist) {
		return []ConnectedDevice{}, nil
	}
	if err != nil {
		return nil, err
	}
	now = now.UTC()
	devices := make([]ConnectedDevice, 0, len(state.Sessions))
	for _, value := range state.Sessions {
		if !value.ExpiresAt.After(now) {
			continue
		}
		pairedAt := value.PairedAt
		if pairedAt.IsZero() {
			pairedAt = value.ExpiresAt.Add(-24 * time.Hour)
		}
		lastSeen := value.LastSeen
		if lastSeen.IsZero() {
			lastSeen = pairedAt
		}
		devices = append(devices, ConnectedDevice{
			Name:      value.Device,
			PairedAt:  pairedAt,
			LastSeen:  lastSeen,
			ExpiresAt: value.ExpiresAt,
		})
	}
	sort.Slice(devices, func(left, right int) bool {
		if devices[left].LastSeen.Equal(devices[right].LastSeen) {
			return strings.ToLower(devices[left].Name) < strings.ToLower(devices[right].Name)
		}
		return devices[left].LastSeen.After(devices[right].LastSeen)
	})
	return devices, nil
}

func setBrowserHeaders(response http.ResponseWriter) {
	response.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'self' wails://wails http://127.0.0.1:* http://localhost:*")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")
}

func (g *Gate) revoke(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	g.mu.Lock()
	g.sessions = map[string]session{}
	err := g.saveLocked()
	g.mu.Unlock()
	if err != nil {
		http.Error(response, "Unable to revoke paired devices.", http.StatusInternalServerError)
		return
	}
	http.SetCookie(response, &http.Cookie{
		Name:     cookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   request.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	http.Redirect(response, request, "/pair", http.StatusSeeOther)
}

func (g *Gate) load() error {
	state, err := readDiskState(g.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if state.Sessions != nil {
		g.sessions = state.Sessions
	}
	g.pruneLocked()
	for len(g.sessions) > maxPairSessions {
		g.removeOldestSessionLocked()
	}
	return nil
}

func readDiskState(path string) (diskState, error) {
	file, err := os.Open(path)
	if err != nil {
		return diskState{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return diskState{}, err
	}
	if info.Size() > maxPairStateSize {
		return diskState{}, errors.New("paired device state is too large")
	}
	var state diskState
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return diskState{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return diskState{}, errors.New("paired device state contains trailing data")
	}
	return state, nil
}

func (g *Gate) pruneLocked() {
	now := g.now().UTC()
	for key, value := range g.sessions {
		if !value.ExpiresAt.After(now) {
			delete(g.sessions, key)
		}
	}
}

func (g *Gate) makeSessionRoomLocked() {
	g.pruneLocked()
	for len(g.sessions) >= maxPairSessions {
		g.removeOldestSessionLocked()
	}
}

func (g *Gate) removeOldestSessionLocked() {
	oldestKey := ""
	var oldestExpiry time.Time
	for key, value := range g.sessions {
		if oldestKey == "" || value.ExpiresAt.Before(oldestExpiry) {
			oldestKey = key
			oldestExpiry = value.ExpiresAt
		}
	}
	if oldestKey != "" {
		delete(g.sessions, oldestKey)
	}
}

func (g *Gate) saveLocked() error {
	g.pruneLocked()
	data, err := json.MarshalIndent(diskState{Sessions: g.sessions}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary := g.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, g.path)
}

func validOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if requestIsLoopback(request) && isLocalAppOrigin(origin) {
		return true
	}
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	parsed, err := url.Parse(origin)
	if err != nil ||
		!strings.EqualFold(parsed.Scheme, scheme) ||
		parsed.User != nil ||
		parsed.Hostname() == "" ||
		(parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return false
	}
	originHost, originPort, ok := normalizedHostPort(parsed.Host, scheme)
	if !ok {
		return false
	}
	requestHost, requestPort, ok := normalizedHostPort(request.Host, scheme)
	if !ok || originPort != requestPort {
		return false
	}
	if strings.EqualFold(originHost, requestHost) {
		return true
	}
	return isLoopbackHost(originHost) && isLoopbackHost(requestHost)
}

func isLocalAppOrigin(origin string) bool {
	if strings.EqualFold(origin, "null") {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil ||
		!strings.EqualFold(parsed.Scheme, "wails") ||
		!strings.EqualFold(parsed.Hostname(), "wails") ||
		parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return false
	}
	return true
}

func normalizedHostPort(value, scheme string) (string, string, bool) {
	parsed, err := url.Parse("//" + value)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" {
		return "", "", false
	}
	port := parsed.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.TrimSuffix(strings.ToLower(parsed.Hostname()), "."), port, true
}

func isLoopbackHost(value string) bool {
	if value == "localhost" {
		return true
	}
	address := net.ParseIP(value)
	return address != nil && address.IsLoopback()
}

func requestIsLoopback(request *http.Request) bool {
	host := requestAddress(request)
	address := net.ParseIP(strings.Trim(host, "[]"))
	return address != nil && address.IsLoopback()
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

var pairTemplate = template.Must(template.New("pair").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pair with {{.Title}}</title>
  <style>
    :root{color-scheme:dark;font-family:Inter,system-ui,sans-serif;font-synthesis:none;-webkit-font-smoothing:antialiased}
    *{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px;background:#191919;color:#f5f5f5}
    main{width:min(100%,420px);padding:24px;border-radius:16px;background:linear-gradient(180deg,#292929b5,#202020b5);box-shadow:inset 0 .1px .2px .1px #ffffff66,0 24px 60px #0006}
    p{color:#b7b7b7;line-height:1.5;text-wrap:pretty}h1{font-size:24px;line-height:1.1;font-weight:600;letter-spacing:-.02em;text-wrap:balance}
    label{display:grid;gap:8px;margin-top:18px;font-size:14px;font-weight:500}input{width:100%;min-height:44px;border:0;border-radius:10px;padding:0 12px;background:#ffffff0d;color:inherit;box-shadow:inset 0 0 0 1px #ffffff1a;font:inherit}
    input:focus-visible,button:focus-visible{outline:2px solid #fff;outline-offset:2px}button{width:100%;min-height:44px;margin-top:20px;border:0;border-radius:10px;background:#f4f4f4;color:#181818;font:500 14px Inter,system-ui,sans-serif;cursor:pointer}
    button:active{transform:scale(.96)}.error{color:#ffb4ad}
  </style>
</head>
<body><main><p>Trusted device</p><h1>Pair with {{.Title}}</h1><p>Enter the six-digit code shown in Spare on this computer. This device stays paired for 24 hours.</p>
{{if .Error}}<p class="error" role="alert">That pairing code did not match. Check the code and try again.</p>{{end}}
<form method="post" action="/pair">
<label for="device">Device name<input id="device" name="device" autocomplete="off" placeholder="My phone"></label>
<label for="code">Pairing code<input id="code" name="code" inputmode="numeric" autocomplete="one-time-code" minlength="6" required placeholder="000000"></label>
<button type="submit">Pair device</button>
</form></main></body></html>`))
