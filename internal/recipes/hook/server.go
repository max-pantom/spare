package hook

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/spare-run/spare/internal/health"
)

const (
	maxRequestBody  = int64(1 << 20)
	maxReplayBody   = int64(64 << 10)
	maxReplayInput  = int64(8 << 10)
	maxHistoryItems = 50
	maxReplayItems  = 20
	replayTimeout   = 15 * time.Second
)

type capturedRequest struct {
	ID            string          `json:"id"`
	Method        string          `json:"method"`
	Path          string          `json:"path"`
	Query         string          `json:"query,omitempty"`
	Host          string          `json:"host"`
	RemoteAddress string          `json:"remoteAddress"`
	Headers       http.Header     `json:"headers"`
	Body          string          `json:"body"`
	BodyEncoding  string          `json:"bodyEncoding,omitempty"`
	BodySize      int             `json:"bodySize"`
	ReceivedAt    time.Time       `json:"receivedAt"`
	Replays       []replayAttempt `json:"replays"`
	rawBody       []byte
}

type requestSummary struct {
	ID            string    `json:"id"`
	Method        string    `json:"method"`
	Path          string    `json:"path"`
	Query         string    `json:"query,omitempty"`
	RemoteAddress string    `json:"remoteAddress"`
	ContentType   string    `json:"contentType,omitempty"`
	BodySize      int       `json:"bodySize"`
	ReceivedAt    time.Time `json:"receivedAt"`
	ReplayCount   int       `json:"replayCount"`
}

type replayAttempt struct {
	ID              string      `json:"id"`
	TargetURL       string      `json:"targetUrl"`
	Status          string      `json:"status"`
	StatusCode      int         `json:"statusCode,omitempty"`
	DurationMS      int64       `json:"durationMs"`
	ResponseHeaders http.Header `json:"responseHeaders,omitempty"`
	ResponseBody    string      `json:"responseBody,omitempty"`
	BodyEncoding    string      `json:"bodyEncoding,omitempty"`
	BodyTruncated   bool        `json:"bodyTruncated,omitempty"`
	Error           string      `json:"error,omitempty"`
	CreatedAt       time.Time   `json:"createdAt"`
}

type server struct {
	mu       sync.RWMutex
	requests []*capturedRequest
	client   *http.Client
}

func newServer() *server {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxResponseHeaderBytes = 64 << 10
	return &server{
		client: &http.Client{
			Timeout:   replayTimeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *server) serve(port, healthPort int) error {
	healthServer, err := health.Start(healthPort, func() health.Snapshot {
		count, latest := s.healthSummary()
		return health.Snapshot{
			Status:     "healthy",
			ItemCount:  count,
			LatestItem: latest,
		}
	})
	if err != nil {
		return err
	}
	defer healthServer.Close()

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", port),
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return httpServer.ListenAndServe()
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/hook", s.capture)
	mux.HandleFunc("/hook/", s.capture)
	mux.HandleFunc("/api/requests", s.list)
	mux.HandleFunc("/api/requests/", s.requestAPI)
	return hookSecurityHeaders(mux)
}

func (s *server) index(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if request.Method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(response, hookPage)
}

func (s *server) capture(response http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, maxRequestBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeHookError(response, http.StatusRequestEntityTooLarge, "Request body is larger than 1 MB.")
			return
		}
		writeHookError(response, http.StatusBadRequest, "Unable to read the request body.")
		return
	}

	bodyValue, encoding := displayBody(body)
	remoteAddress, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		remoteAddress = request.RemoteAddr
	}
	captured := &capturedRequest{
		ID:            newID(),
		Method:        request.Method,
		Path:          request.URL.EscapedPath(),
		Query:         request.URL.RawQuery,
		Host:          request.Host,
		RemoteAddress: remoteAddress,
		Headers:       request.Header.Clone(),
		Body:          bodyValue,
		BodyEncoding:  encoding,
		BodySize:      len(body),
		ReceivedAt:    time.Now().UTC(),
		Replays:       []replayAttempt{},
		rawBody:       append([]byte(nil), body...),
	}
	s.add(captured)

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("X-Spare-Hook-Request-ID", captured.ID)
	response.WriteHeader(http.StatusAccepted)
	if request.Method != http.MethodHead {
		_ = json.NewEncoder(response).Encode(map[string]any{
			"id":       captured.ID,
			"received": true,
		})
	}
}

func (s *server) list(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", "GET")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(map[string]any{"requests": s.summaries()})
}

func (s *server) requestAPI(response http.ResponseWriter, request *http.Request) {
	relative := strings.TrimPrefix(request.URL.Path, "/api/requests/")
	parts := strings.Split(strings.Trim(relative, "/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", "GET")
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		captured, ok := s.get(parts[0])
		if !ok {
			writeHookError(response, http.StatusNotFound, "Request not found.")
			return
		}
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(response).Encode(captured)
		return
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "replay" {
		s.replay(response, request, parts[0])
		return
	}
	http.NotFound(response, request)
}

func (s *server) replay(response http.ResponseWriter, request *http.Request, id string) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !sameOrigin(request) {
		writeHookError(response, http.StatusForbidden, "Replay requests must come from this Hook page.")
		return
	}
	captured, ok := s.getInternal(id)
	if !ok {
		writeHookError(response, http.StatusNotFound, "Request not found.")
		return
	}
	var input struct {
		TargetURL string `json:"targetUrl"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, maxReplayInput))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeHookError(response, http.StatusBadRequest, "Enter a replay URL.")
		return
	}
	target, err := validateTargetURL(input.TargetURL)
	if err != nil {
		writeHookError(response, http.StatusBadRequest, err.Error())
		return
	}

	result := s.sendReplay(request.Context(), captured, target)
	s.addReplay(id, result)
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(map[string]any{"replay": result})
}

func (s *server) sendReplay(ctx context.Context, captured *capturedRequest, target string) replayAttempt {
	started := time.Now()
	result := replayAttempt{
		ID:        newID(),
		TargetURL: target,
		Status:    "failed",
		CreatedAt: started.UTC(),
	}
	outbound, err := http.NewRequestWithContext(ctx, captured.Method, target, bytes.NewReader(captured.rawBody))
	if err != nil {
		result.Error = "Unable to create the replay request."
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	copyReplayHeaders(outbound.Header, captured.Headers)
	outbound.Header.Set("X-Spare-Hook-Replay", captured.ID)

	replayed, err := s.client.Do(outbound)
	result.DurationMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = replayError(err)
		return result
	}
	defer replayed.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(replayed.Body, maxReplayBody+1))
	if readErr != nil {
		result.Error = "The destination responded, but Hook could not read its response."
		return result
	}
	if int64(len(responseBody)) > maxReplayBody {
		responseBody = responseBody[:maxReplayBody]
		result.BodyTruncated = true
	}
	result.Status = "completed"
	result.StatusCode = replayed.StatusCode
	result.ResponseHeaders = replayed.Header.Clone()
	result.ResponseBody, result.BodyEncoding = displayBody(responseBody)
	return result
}

func (s *server) add(captured *capturedRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append([]*capturedRequest{captured}, s.requests...)
	if len(s.requests) > maxHistoryItems {
		s.requests = s.requests[:maxHistoryItems]
	}
}

func (s *server) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.requests)
}

func (s *server) healthSummary() (int, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.requests) == 0 {
		return 0, ""
	}
	latest := s.requests[0]
	return len(s.requests), latest.Method + " " + latest.Path
}

func (s *server) summaries() []requestSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]requestSummary, 0, len(s.requests))
	for _, captured := range s.requests {
		result = append(result, requestSummary{
			ID:            captured.ID,
			Method:        captured.Method,
			Path:          captured.Path,
			Query:         captured.Query,
			RemoteAddress: captured.RemoteAddress,
			ContentType:   captured.Headers.Get("Content-Type"),
			BodySize:      captured.BodySize,
			ReceivedAt:    captured.ReceivedAt,
			ReplayCount:   len(captured.Replays),
		})
	}
	return result
}

func (s *server) get(id string) (*capturedRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, captured := range s.requests {
		if captured.ID == id {
			return cloneCaptured(captured), true
		}
	}
	return nil, false
}

func (s *server) getInternal(id string) (*capturedRequest, bool) {
	return s.get(id)
}

func (s *server) addReplay(id string, replay replayAttempt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, captured := range s.requests {
		if captured.ID == id {
			captured.Replays = append([]replayAttempt{replay}, captured.Replays...)
			if len(captured.Replays) > maxReplayItems {
				captured.Replays = captured.Replays[:maxReplayItems]
			}
			return
		}
	}
}

func cloneCaptured(captured *capturedRequest) *capturedRequest {
	cloned := *captured
	cloned.Headers = captured.Headers.Clone()
	cloned.rawBody = append([]byte(nil), captured.rawBody...)
	cloned.Replays = make([]replayAttempt, len(captured.Replays))
	copy(cloned.Replays, captured.Replays)
	for index := range cloned.Replays {
		cloned.Replays[index].ResponseHeaders = cloned.Replays[index].ResponseHeaders.Clone()
	}
	return &cloned
}

func displayBody(body []byte) (string, string) {
	if len(body) == 0 {
		return "", ""
	}
	if utf8.Valid(body) {
		return string(body), ""
	}
	return base64.StdEncoding.EncodeToString(body), "base64"
}

func validateTargetURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 2048 {
		return "", errors.New("Use a replay URL shorter than 2,048 characters.")
	}
	if strings.Contains(trimmed, "#") {
		return "", errors.New("Remove the fragment from the replay URL.")
	}
	target, err := url.ParseRequestURI(trimmed)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return "", errors.New("Use a full replay URL beginning with http:// or https://.")
	}
	if target.User != nil {
		return "", errors.New("Put credentials in the captured request headers, not the replay URL.")
	}
	if target.Fragment != "" {
		return "", errors.New("Remove the fragment from the replay URL.")
	}
	return target.String(), nil
}

func copyReplayHeaders(destination, source http.Header) {
	for key, values := range source {
		if hopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}

func hopByHopHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Connection", "Content-Length", "Host", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, request.Host)
}

func replayError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "The destination did not respond within 15 seconds."
	}
	return "Unable to reach the replay destination."
}

func newID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func writeHookError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]any{
		"error": map[string]string{"message": message},
	})
}

func hookSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(response, request)
	})
}
