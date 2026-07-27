package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/state"
	"github.com/spare-run/spare/internal/supervisor"
)

const sessionCookie = "spare_session"

type Server struct {
	token     string
	store     *state.Store
	manager   *supervisor.Manager
	assets    fs.FS
	sessions  *sessionStore
	fileServe http.Handler
}

type createInstanceRequest struct {
	RecipeID string         `json:"recipeId"`
	Mode     string         `json:"mode"`
	Config   map[string]any `json:"config"`
	Port     int            `json:"port"`
	PortMode string         `json:"portMode"`
}

func NewServer(token string, store *state.Store, manager *supervisor.Manager, assets fs.FS) *Server {
	return &Server{
		token:     token,
		store:     store,
		manager:   manager,
		assets:    assets,
		sessions:  newSessionStore(),
		fileServe: http.FileServer(http.FS(assets)),
	}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Frame-Options", "DENY")

	if request.URL.Path == "/auth/exchange" {
		s.exchange(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v1/") {
		authKind, ok := s.authenticate(request)
		if !ok {
			writeAPIError(response, http.StatusUnauthorized, "authentication_required", "Authentication is required.", "Open the dashboard with `spare open dashboard`.")
			return
		}
		if isMutation(request.Method) && authKind == "cookie" && !validOrigin(request) {
			writeAPIError(response, http.StatusForbidden, "invalid_origin", "This request did not come from the Spare dashboard.", "Open the dashboard with `spare open dashboard`.")
			return
		}
		s.serveAPI(response, request, authKind)
		return
	}
	if !s.validSession(request) {
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Open Spare</title></head><body><main><h1>Open Spare from the CLI</h1><p>Run <code>spare open dashboard</code> to create a secure local browser session.</p></main></body></html>`))
		return
	}
	s.serveDashboard(response, request)
}

func (s *Server) authenticate(request *http.Request) (string, bool) {
	if value := request.Header.Get("Authorization"); strings.HasPrefix(value, "Bearer ") {
		return "bearer", strings.TrimPrefix(value, "Bearer ") == s.token
	}
	if s.validSession(request) {
		return "cookie", true
	}
	return "", false
}

func (s *Server) validSession(request *http.Request) bool {
	cookie, err := request.Cookie(sessionCookie)
	return err == nil && s.sessions.Valid(cookie.Value)
}

func (s *Server) exchange(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := s.sessions.Exchange(request.URL.Query().Get("code"))
	if !ok {
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Session expired</title></head><body><main><h1>This dashboard link has expired</h1><p>Run <code>spare open dashboard</code> to create a new link.</p></main></body></html>`))
		return
	}
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookie,
		Value:    session,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   12 * 60 * 60,
	})
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *Server) serveAPI(response http.ResponseWriter, request *http.Request, authKind string) {
	path := strings.TrimPrefix(request.URL.Path, "/api/v1")
	if strings.HasPrefix(path, "/desktop/") && authKind != "bearer" {
		writeAPIError(response, http.StatusForbidden, "desktop_only", "This operation is available only in Spare Desktop.", "Open Spare on the computer itself.")
		return
	}
	if authKind != "bearer" && requiresLocalBearer(path, request.Method) {
		writeAPIError(response, http.StatusForbidden, "local_user_required", "This operation must be approved on the Spare computer.", "Use Spare Desktop or the local `spare` CLI.")
		return
	}
	switch {
	case path == "/health" && request.Method == http.MethodGet:
		writeJSON(response, http.StatusOK, map[string]string{"status": "healthy"})
	case path == "/machine" && request.Method == http.MethodGet:
		machine, err := s.store.Machine(request.Context())
		if err != nil {
			writeInternalError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, machine)
	case path == "/recipes" && request.Method == http.MethodGet:
		writeJSON(response, http.StatusOK, s.manager.Recipes())
	case path == "/instances" && request.Method == http.MethodGet:
		writeJSON(response, http.StatusOK, s.manager.List())
	case path == "/instances" && request.Method == http.MethodPost:
		s.createInstance(response, request)
	case path == "/events" && request.Method == http.MethodGet:
		limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
		events, err := s.store.Events(request.Context(), limit)
		if err != nil {
			writeInternalError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, events)
	case path == "/activity/stream" && request.Method == http.MethodGet:
		s.streamActivity(response, request)
	case path == "/desktop/backups/export" && request.Method == http.MethodPost:
		s.exportBackup(response, request)
	case path == "/desktop/backups/restore" && request.Method == http.MethodPost:
		s.restoreBackup(response, request)
	case path == "/desktop/drop-files" && request.Method == http.MethodPost:
		s.addDropFiles(response, request)
	case path == "/browser-sessions" && request.Method == http.MethodPost:
		code := s.sessions.NewCode()
		url := fmt.Sprintf("http://%s/auth/exchange?code=%s", request.Host, code)
		writeJSON(response, http.StatusCreated, map[string]string{"url": url})
	case strings.HasPrefix(path, "/instances/"):
		s.instanceAction(response, request, strings.TrimPrefix(path, "/instances/"))
	default:
		writeAPIError(response, http.StatusNotFound, "endpoint_not_found", "The requested API endpoint does not exist.", "")
	}
}

func requiresLocalBearer(path, method string) bool {
	if path == "/instances" && method == http.MethodPost {
		return true
	}
	if !strings.HasPrefix(path, "/instances/") {
		return false
	}
	if method == http.MethodDelete {
		return true
	}
	return method == http.MethodPost &&
		(strings.HasSuffix(path, "/configure") ||
			strings.HasSuffix(path, "/heartbeat") ||
			strings.HasSuffix(path, "/promote"))
}

func (s *Server) exportBackup(response http.ResponseWriter, request *http.Request) {
	var input struct {
		InstanceID  string `json:"instanceId"`
		Destination string `json:"destination"`
	}
	if !decodeDesktopRequest(response, request, &input) {
		return
	}
	if input.InstanceID == "" || input.Destination == "" {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "Choose a recipe and backup destination.", "Use the desktop backup picker and try again.")
		return
	}
	if err := s.manager.ExportBackup(input.InstanceID, input.Destination); err != nil {
		writeManagerError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]string{"destination": input.Destination})
}

func (s *Server) restoreBackup(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	if !decodeDesktopRequest(response, request, &input) {
		return
	}
	if input.Source == "" || input.Destination == "" {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "Choose a backup and an empty destination folder.", "Use both desktop pickers and try again.")
		return
	}
	instance, err := s.manager.RestoreBackup(input.Source, input.Destination)
	if err != nil {
		writeManagerError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, instance)
}

func (s *Server) addDropFiles(response http.ResponseWriter, request *http.Request) {
	var input struct {
		InstanceID string   `json:"instanceId"`
		Paths      []string `json:"paths"`
	}
	if !decodeDesktopRequest(response, request, &input) {
		return
	}
	names, err := s.manager.AddDropFiles(input.InstanceID, input.Paths)
	if err != nil {
		writeManagerError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"names": names})
}

func decodeDesktopRequest(response http.ResponseWriter, request *http.Request, output any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 128*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "The desktop request is invalid.", err.Error())
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "The desktop request is invalid.", "Send exactly one JSON object.")
		return false
	}
	return true
}

func (s *Server) createInstance(response http.ResponseWriter, request *http.Request) {
	var input createInstanceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "The recipe configuration is invalid.", err.Error())
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "The recipe configuration is invalid.", "Send exactly one JSON object.")
		return
	}
	instance, err := s.manager.Create(supervisor.CreateRequest{
		RecipeID: input.RecipeID,
		Mode:     input.Mode,
		Config:   input.Config,
		Port:     input.Port,
		PortMode: input.PortMode,
	})
	if err != nil {
		writeManagerError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, instance)
}

func (s *Server) instanceAction(response http.ResponseWriter, request *http.Request, route string) {
	parts := strings.Split(strings.Trim(route, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(response, http.StatusNotFound, "instance_not_found", "The requested recipe is not installed.", "Run `spare status` to list installed recipes.")
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		switch request.Method {
		case http.MethodGet:
			instance, err := s.manager.Get(id)
			if err != nil {
				writeManagerError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, instance)
		case http.MethodDelete:
			if err := s.manager.Remove(id); err != nil {
				writeManagerError(response, err)
				return
			}
			response.WriteHeader(http.StatusNoContent)
		default:
			response.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) != 2 || request.Method != http.MethodPost {
		writeAPIError(response, http.StatusNotFound, "endpoint_not_found", "The requested API endpoint does not exist.", "")
		return
	}

	var (
		instance model.Instance
		err      error
	)
	switch parts[1] {
	case "start":
		instance, err = s.manager.Start(id)
	case "stop":
		instance, err = s.manager.Stop(id)
	case "configure":
		var input createInstanceRequest
		if !decodeDesktopRequest(response, request, &input) {
			return
		}
		instance, err = s.manager.Configure(id, supervisor.CreateRequest{
			RecipeID: input.RecipeID,
			Config:   input.Config,
			Port:     input.Port,
			PortMode: input.PortMode,
		})
	case "heartbeat":
		err = s.manager.Heartbeat(id)
		if err == nil {
			response.WriteHeader(http.StatusNoContent)
			return
		}
	case "promote":
		instance, err = s.manager.Promote(id)
	default:
		writeAPIError(response, http.StatusNotFound, "endpoint_not_found", "The requested API endpoint does not exist.", "")
		return
	}
	if err != nil {
		writeManagerError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, instance)
}

func (s *Server) streamActivity(response http.ResponseWriter, request *http.Request) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		writeAPIError(response, http.StatusNotImplemented, "streaming_unavailable", "Live activity is unavailable.", "Refresh the activity view.")
		return
	}
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(response, ": connected\n\n")
	flusher.Flush()

	events := s.store.SubscribeEvents(request.Context())
	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(response, "event: activity\ndata: %s\n\n", data)
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = io.WriteString(response, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) serveDashboard(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	filePath := strings.TrimPrefix(request.URL.Path, "/")
	if filePath == "" {
		filePath = "index.html"
	}
	if _, err := fs.Stat(s.assets, filePath); err != nil {
		request.URL.Path = "/"
	}
	s.fileServe.ServeHTTP(response, request)
}

func validOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return origin == "http://"+request.Host
}

func isMutation(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func writeManagerError(response http.ResponseWriter, err error) {
	var managerError *supervisor.ManagerError
	if errors.As(err, &managerError) {
		status := http.StatusBadRequest
		if strings.Contains(managerError.Code, "not_found") {
			status = http.StatusNotFound
		} else if managerError.Code == "role_already_exists" || managerError.Code == "port_in_use" {
			status = http.StatusConflict
		} else if managerError.Code == "daemon_stopping" {
			status = http.StatusServiceUnavailable
		}
		writeAPIError(response, status, managerError.Code, managerError.Message, managerError.Hint)
		return
	}
	writeInternalError(response, err)
}

func writeInternalError(response http.ResponseWriter, err error) {
	writeAPIError(response, http.StatusInternalServerError, "internal_error", "Spare could not complete this request.", err.Error())
}

func writeAPIError(response http.ResponseWriter, status int, code, message, hint string) {
	writeJSON(response, status, model.ErrorEnvelope{
		Error: model.APIError{Code: code, Message: message, Hint: hint},
	})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
