package monitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spare-run/spare/internal/health"
	"github.com/spare-run/spare/internal/recipes/shared/pairing"
	"github.com/spare-run/spare/internal/recipes/shared/webui"
)

type checkResult struct {
	Status     string    `json:"status"`
	ResponseMS int64     `json:"responseMs"`
	Message    string    `json:"message,omitempty"`
	CheckedAt  time.Time `json:"checkedAt"`
}

type target struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Address     string        `json:"address"`
	Status      string        `json:"status"`
	ResponseMS  int64         `json:"responseMs"`
	Message     string        `json:"message,omitempty"`
	LastCheck   *time.Time    `json:"lastCheck,omitempty"`
	LastSuccess *time.Time    `json:"lastSuccess,omitempty"`
	History     []checkResult `json:"history"`
}

type server struct {
	mu        sync.Mutex
	root      string
	statePath string
	interval  time.Duration
	targets   []target
	gate      *pairing.Gate
	client    *http.Client
	checkNow  chan struct{}
}

const (
	maxMonitorTargets    = 100
	maxMonitorStateBytes = 4 * 1024 * 1024
)

var hostPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,252}$`)

func newServer(values map[string]any, dataPath string) (*server, error) {
	code, _ := values["pairing-code"].(string)
	seconds, err := monitorInteger(values["check-interval"])
	if err != nil {
		return nil, err
	}
	if seconds <= 0 {
		seconds = 30
	}
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dataPath, 0o700); err != nil {
		return nil, err
	}
	gate, err := pairing.New(code, "Monitor", dataPath)
	if err != nil {
		return nil, err
	}
	value := &server{
		root:      dataPath,
		statePath: filepath.Join(dataPath, "monitor.json"),
		interval:  time.Duration(seconds) * time.Second,
		gate:      gate,
		client:    newMonitorHTTPClient(),
		checkNow:  make(chan struct{}, 1),
	}
	if err := value.load(); err != nil {
		return nil, err
	}
	return value, nil
}

func newMonitorHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   4 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           8,
		MaxIdleConnsPerHost:    2,
		IdleConnTimeout:        60 * time.Second,
		TLSHandshakeTimeout:    4 * time.Second,
		ResponseHeaderTimeout:  4 * time.Second,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 64 * 1024,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if err := validateMonitorHTTPURL(request.URL); err != nil {
				return err
			}
			if len(via) > 0 &&
				via[len(via)-1].URL.Scheme == "https" &&
				request.URL.Scheme != "https" {
				return errors.New("Monitor refused an insecure HTTPS redirect")
			}
			request.Header.Del("Referer")
			return nil
		},
	}
}

func (s *server) serve(port, healthPort int) error {
	healthServer, err := health.Start(healthPort, s.health)
	if err != nil {
		return err
	}
	defer healthServer.Close()
	go s.runChecks()
	s.signal()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/targets", s.add)
	mux.HandleFunc("/targets/", s.action)
	mux.HandleFunc("/check", s.check)
	handler := monitorSecurityHeaders(s.gate.Middleware(mux))
	httpServer := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 * 1024,
	}
	return httpServer.ListenAndServe()
}

func (s *server) health() health.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	down := 0
	for _, candidate := range s.targets {
		if candidate.Status == "down" {
			down++
		}
	}
	if down > 0 {
		return health.Snapshot{
			Status:          "degraded",
			ProblemCode:     "monitor_target_down",
			ProblemSummary:  fmt.Sprintf("%d monitored %s offline.", down, plural(down, "target is", "targets are")),
			ProblemRecovery: "Monitor keeps checking and will report when every target recovers.",
		}
	}
	return health.Snapshot{Status: "healthy"}
}

func (s *server) home(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", "GET")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	targets := append([]target(nil), s.targets...)
	s.mu.Unlock()
	sort.Slice(targets, func(left, right int) bool {
		return strings.ToLower(targets[left].Name) < strings.ToLower(targets[right].Name)
	})
	data := struct {
		Styles   template.CSS
		Targets  []target
		Interval string
	}{
		Styles:   template.CSS(webui.Styles),
		Targets:  targets,
		Interval: s.interval.String(),
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if err := monitorTemplate.Execute(response, data); err != nil {
		http.Error(response, "Unable to show Monitor.", http.StatusInternalServerError)
	}
}

func (s *server) add(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 16*1024)
	if err := request.ParseForm(); err != nil {
		http.Error(response, "Unable to read this monitor.", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(request.FormValue("name"))
	kind := strings.TrimSpace(request.FormValue("type"))
	address := strings.TrimSpace(request.FormValue("address"))
	if name == "" || len([]rune(name)) > 80 {
		http.Error(response, "Enter a name using 80 characters or fewer.", http.StatusBadRequest)
		return
	}
	normalized, err := validateTarget(kind, address)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := targetID()
	if err != nil {
		http.Error(response, "Unable to create this monitor.", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	if len(s.targets) >= maxMonitorTargets {
		s.mu.Unlock()
		http.Error(response, "Monitor can track no more than 100 targets.", http.StatusConflict)
		return
	}
	s.targets = append(s.targets, target{
		ID:      id,
		Name:    name,
		Type:    kind,
		Address: normalized,
		Status:  "unknown",
	})
	err = s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		http.Error(response, "Unable to save this monitor.", http.StatusInternalServerError)
		return
	}
	s.signal()
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) action(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !strings.HasSuffix(request.URL.Path, "/delete") {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/targets/"), "/delete")
	s.mu.Lock()
	for index := range s.targets {
		if s.targets[index].ID == id {
			s.targets = append(s.targets[:index], s.targets[index+1:]...)
			break
		}
	}
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		http.Error(response, "Unable to remove this monitor.", http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) check(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.signal()
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) runChecks() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-s.checkNow:
		}
		s.checkAll()
	}
}

func (s *server) checkAll() {
	s.mu.Lock()
	targets := append([]target(nil), s.targets...)
	s.mu.Unlock()
	for _, candidate := range targets {
		result := s.runCheck(candidate)
		s.mu.Lock()
		for index := range s.targets {
			if s.targets[index].ID != candidate.ID {
				continue
			}
			s.targets[index].Status = result.Status
			s.targets[index].ResponseMS = result.ResponseMS
			s.targets[index].Message = result.Message
			checked := result.CheckedAt
			s.targets[index].LastCheck = &checked
			if result.Status == "up" {
				s.targets[index].LastSuccess = &checked
			}
			s.targets[index].History = append(s.targets[index].History, result)
			if len(s.targets[index].History) > 60 {
				s.targets[index].History = append([]checkResult(nil), s.targets[index].History[len(s.targets[index].History)-60:]...)
			}
			break
		}
		_ = s.saveLocked()
		s.mu.Unlock()
	}
}

func (s *server) runCheck(candidate target) checkResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	switch candidate.Type {
	case "http":
		var request *http.Request
		request, err = http.NewRequestWithContext(ctx, http.MethodGet, candidate.Address, nil)
		if err == nil {
			var response *http.Response
			response, err = s.client.Do(request)
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode < 200 || response.StatusCode >= 400 {
					err = fmt.Errorf("HTTP %d", response.StatusCode)
				}
			} else {
				err = safeHTTPCheckError(err)
			}
		}
	case "tcp":
		var connection net.Conn
		dialer := net.Dialer{Timeout: 5 * time.Second}
		connection, err = dialer.DialContext(ctx, "tcp", candidate.Address)
		if err == nil {
			_ = connection.Close()
		}
	case "ping":
		err = ping(ctx, candidate.Address)
	default:
		err = errors.New("unsupported monitor type")
	}
	result := checkResult{
		Status:     "up",
		ResponseMS: time.Since(started).Milliseconds(),
		CheckedAt:  time.Now().UTC(),
	}
	if err != nil {
		result.Status = "down"
		result.Message = compactError(err)
	}
	return result
}

func validateTarget(kind, address string) (string, error) {
	switch kind {
	case "http":
		if len(address) > 2048 {
			return "", errors.New("HTTP addresses can contain up to 2,048 characters")
		}
		parsed, err := url.Parse(address)
		if err != nil {
			return "", errors.New("enter a complete HTTP or HTTPS address")
		}
		if err := validateMonitorHTTPURL(parsed); err != nil {
			return "", err
		}
		if parsed.Fragment != "" {
			return "", errors.New("HTTP monitor addresses cannot contain fragments")
		}
		return parsed.String(), nil
	case "tcp":
		if len(address) > 512 {
			return "", errors.New("TCP addresses can contain up to 512 characters")
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil || host == "" || port == "" {
			return "", errors.New("enter a host and port, such as printer.local:9100")
		}
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", errors.New("enter a numeric TCP port")
		}
		return net.JoinHostPort(host, port), nil
	case "ping":
		if !hostPattern.MatchString(address) || strings.HasPrefix(address, "-") {
			return "", errors.New("enter a hostname or IP address")
		}
		return address, nil
	default:
		return "", errors.New("choose HTTP, ping, or TCP port")
	}
}

func validateMonitorHTTPURL(address *url.URL) error {
	if address == nil ||
		address.Host == "" ||
		(address.Scheme != "http" && address.Scheme != "https") {
		return errors.New("unsupported monitor address")
	}
	if address.User != nil {
		return errors.New("addresses containing usernames or passwords are not supported")
	}
	return nil
}

func safeHTTPCheckError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("The HTTP check timed out.")
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return errors.New("The HTTP check timed out.")
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return errors.New("The hostname could not be resolved.")
	}
	return errors.New("The HTTP connection failed.")
}

func ping(ctx context.Context, host string) error {
	var arguments []string
	switch runtime.GOOS {
	case "windows":
		arguments = []string{"-n", "1", "-w", "4000", host}
	case "darwin":
		arguments = []string{"-c", "1", "-W", "4000", host}
	default:
		arguments = []string{"-c", "1", "-W", "4", host}
	}
	output, err := exec.CommandContext(ctx, "ping", arguments...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return errors.New(compactError(errors.New(message)))
		}
	}
	return err
}

func compactError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.Join(strings.Fields(err.Error()), " ")
	if len(value) > 160 {
		value = value[:157] + "…"
	}
	return value
}

func (s *server) load() error {
	file, err := os.Open(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxMonitorStateBytes {
		return errors.New("Monitor state is too large")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s.targets); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Monitor state contains trailing data")
	}
	if len(s.targets) > maxMonitorTargets {
		return fmt.Errorf("Monitor state contains more than %d targets", maxMonitorTargets)
	}
	seen := make(map[string]bool, len(s.targets))
	for _, candidate := range s.targets {
		if !validTargetID(candidate.ID) || seen[candidate.ID] {
			return errors.New("Monitor state contains an invalid or duplicate target ID")
		}
		seen[candidate.ID] = true
		if len([]rune(candidate.Name)) == 0 || len([]rune(candidate.Name)) > 80 {
			return errors.New("Monitor state contains an invalid target name")
		}
		normalized, err := validateTarget(candidate.Type, candidate.Address)
		if err != nil || normalized != candidate.Address {
			return errors.New("Monitor state contains an invalid target address")
		}
		if len(candidate.History) > 60 || len(candidate.Message) > 160 {
			return errors.New("Monitor state contains oversized history")
		}
		for _, result := range candidate.History {
			if result.Status != "up" && result.Status != "down" {
				return errors.New("Monitor state contains an invalid check result")
			}
			if len(result.Message) > 160 {
				return errors.New("Monitor state contains an oversized check result")
			}
		}
	}
	return nil
}

func (s *server) saveLocked() error {
	data, err := json.MarshalIndent(s.targets, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary := s.statePath + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, s.statePath)
}

func (s *server) signal() {
	select {
	case s.checkNow <- struct{}{}:
	default:
	}
}

func targetID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func validTargetID(value string) bool {
	if len(value) != 24 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 12
}

func monitorInteger(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case float64:
		return int64(typed), nil
	default:
		return 0, fmt.Errorf("invalid integer value %v", value)
	}
}

func monitorSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'self' wails://wails http://127.0.0.1:* http://localhost:*; base-uri 'none'")
		next.ServeHTTP(response, request)
	})
}

func plural(count int, singular, pluralValue string) string {
	if count == 1 {
		return singular
	}
	return pluralValue
}

var monitorTemplate = template.Must(template.New("monitor").Funcs(template.FuncMap{
	"timeLabel": func(value *time.Time) string {
		if value == nil {
			return "Not checked"
		}
		return value.Local().Format("Jan 2, 3:04 PM")
	},
	"displayAddress": displayMonitorAddress,
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Monitor</title><style>{{.Styles}}</style></head>
<body><main class="shell"><header><div><p class="eyebrow">Local monitor</p><h1>Services and devices</h1><p class="subtitle">Check websites, computers, and local ports every {{.Interval}}. Spare reports failures and recoveries through its activity and notifications.</p></div><div class="actions"><form method="post" action="/check"><button type="submit">Check now</button></form><form method="post" action="/pair/revoke"><button class="secondary" type="submit">Revoke devices</button></form></div></header>
<section class="grid"><form class="card" method="post" action="/targets"><h2>Add a monitor</h2><label class="field" for="name">Name<input id="name" name="name" required placeholder="Home internet"></label><label class="field" for="type">Check type<select id="type" name="type"><option value="http">HTTP</option><option value="ping">Ping</option><option value="tcp">TCP port</option></select></label><label class="field" for="address">Address<input id="address" name="address" required placeholder="https://example.com"></label><button type="submit">Add monitor</button></form>
<section class="card stack" aria-labelledby="targets-heading"><h2 id="targets-heading">Current status</h2>{{if .Targets}}{{range .Targets}}<article class="card stack"><div class="row"><div><h2>{{.Name}}</h2><p class="meta" style="overflow-wrap:anywhere">{{.Type}} · {{displayAddress .Address}}</p></div><span class="status {{if eq .Status "up"}}good{{else if eq .Status "down"}}bad{{end}}">{{.Status}}</span></div><div class="row"><span class="meta">{{if .ResponseMS}}{{.ResponseMS}} ms{{else}}No response time{{end}}</span><span class="meta">Last checked {{timeLabel .LastCheck}}</span></div>{{if .Message}}<p class="danger" role="status">{{.Message}}</p>{{end}}<form method="post" action="/targets/{{.ID}}/delete"><button class="secondary" type="submit">Remove monitor</button></form></article>{{end}}{{else}}<div class="empty"><h2>No monitors yet</h2><p>Add a website, device, or TCP port to start checking its availability.</p></div>{{end}}</section></section>
</main></body></html>`))

func displayMonitorAddress(value string) string {
	if parsed, err := url.Parse(value); err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.RawQuery != "" {
		parsed.RawQuery = "redacted"
		return parsed.String()
	}
	return value
}
