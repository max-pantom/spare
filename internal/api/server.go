package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

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
	RecipeID string `json:"recipeId"`
	Mode     string `json:"mode"`
	Config   struct {
		RootPath string `json:"rootPath"`
		Port     int    `json:"port"`
		PortMode string `json:"portMode"`
	} `json:"config"`
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
		s.serveAPI(response, request)
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

func (s *Server) serveAPI(response http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/api/v1")
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
		writeJSON(response, http.StatusOK, []model.Recipe{model.SiteRecipe()})
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

func (s *Server) createInstance(response http.ResponseWriter, request *http.Request) {
	var input createInstanceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "The Site configuration is invalid.", err.Error())
		return
	}
	if input.RecipeID != model.RecipeSite {
		writeAPIError(response, http.StatusBadRequest, "unknown_recipe", "Only the built-in Site recipe is available.", "")
		return
	}
	instance, err := s.manager.Create(supervisor.CreateRequest{
		Mode:     input.Mode,
		RootPath: input.Config.RootPath,
		Port:     input.Config.Port,
		PortMode: input.Config.PortMode,
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
		writeAPIError(response, http.StatusNotFound, "instance_not_found", "Site is not installed.", "")
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
	case "heartbeat":
		err = s.manager.Heartbeat(id)
		if err == nil {
			response.WriteHeader(http.StatusNoContent)
			return
		}
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
