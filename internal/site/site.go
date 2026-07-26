package site

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var ErrOutsideRoot = errors.New("requested file is outside the site root")

func ValidateRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve site folder: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("open site folder: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("open site folder: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("the site path must be a folder")
	}
	return filepath.Clean(resolved), nil
}

type Handler struct {
	root string
}

func NewHandler(root string) (*Handler, error) {
	validated, err := ValidateRoot(root)
	if err != nil {
		return nil, err
	}
	return &Handler{root: validated}, nil
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath, info, err := h.resolve(request.URL.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrOutsideRoot) {
			http.NotFound(response, request)
			return
		}
		http.Error(response, "Unable to read this file", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.NotFound(response, request)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer file.Close()

	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType := mime.TypeByExtension(filepath.Ext(filePath)); contentType != "" {
		response.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(response, request, info.Name(), info.ModTime(), file)
}

func (h *Handler) resolve(urlPath string) (string, os.FileInfo, error) {
	cleaned := path.Clean("/" + urlPath)
	relativeURL := strings.TrimPrefix(cleaned, "/")
	for _, segment := range strings.Split(relativeURL, "/") {
		if strings.HasPrefix(segment, ".") && segment != "" {
			return "", nil, ErrOutsideRoot
		}
	}

	candidate := filepath.Join(h.root, filepath.FromSlash(relativeURL))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", nil, err
	}
	if !within(h.root, resolved) {
		return "", nil, ErrOutsideRoot
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, err
	}
	if info.IsDir() {
		resolved, err = filepath.EvalSymlinks(filepath.Join(resolved, "index.html"))
		if err != nil {
			return "", nil, err
		}
		if !within(h.root, resolved) {
			return "", nil, ErrOutsideRoot
		}
		info, err = os.Stat(resolved)
		if err != nil {
			return "", nil, err
		}
	}
	return resolved, info, nil
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func Serve(root string, port, healthPort int) error {
	handler, err := NewHandler(root)
	if err != nil {
		return err
	}

	health := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", healthPort),
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(response, `{"status":"healthy"}`)
		}),
	}
	go func() {
		_ = health.ListenAndServe()
	}()

	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}
