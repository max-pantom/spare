package recipeview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/spare-run/spare/internal/artifacts"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/recipe"
)

const (
	maxPreviewSize = uint64(2 << 20)
	idleTimeout    = 2 * time.Minute
)

type File struct {
	Name           string `json:"name"`
	Size           uint64 `json:"size"`
	CompressedSize uint64 `json:"compressedSize"`
	Preview        string `json:"preview"`
	MediaType      string `json:"mediaType,omitempty"`
}

type Summary struct {
	FileName         string                  `json:"fileName"`
	PackageSize      int64                   `json:"packageSize"`
	UncompressedSize uint64                  `json:"uncompressedSize"`
	SHA256           string                  `json:"sha256"`
	Manifest         recipe.Manifest         `json:"manifest"`
	Compatibility    model.Compatibility     `json:"compatibility"`
	Permissions      []model.PermissionGrant `json:"permissions"`
	Files            []File                  `json:"files"`
}

type Viewer struct {
	path    string
	summary Summary
	files   map[string]File
}

type Running struct {
	URL          string
	server       *http.Server
	done         chan error
	lastActivity atomic.Int64
}

func New(source string) (*Viewer, error) {
	if !strings.EqualFold(filepath.Ext(source), ".sp") {
		return nil, errors.New("recipe viewer requires a .sp package")
	}
	absolute, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("open recipe package: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("open recipe package: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("recipe package must be a regular file")
	}
	manifest, err := recipe.Load(resolved)
	if err != nil {
		return nil, err
	}
	packageFiles, err := artifacts.ListFiles(resolved)
	if err != nil {
		return nil, err
	}
	checksum, err := artifacts.SHA256(resolved)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(packageFiles))
	byName := make(map[string]File, len(packageFiles))
	var uncompressed uint64
	for _, packageFile := range packageFiles {
		if ^uint64(0)-uncompressed < packageFile.Size {
			return nil, errors.New("package size is too large")
		}
		uncompressed += packageFile.Size
		file := File{
			Name:           packageFile.Name,
			Size:           packageFile.Size,
			CompressedSize: packageFile.CompressedSize,
		}
		file.Preview, file.MediaType = previewType(file.Name, file.Size)
		files = append(files, file)
		byName[file.Name] = file
	}
	statements := permissions.Describe(manifest.Permissions)
	grants := make([]model.PermissionGrant, 0, len(statements))
	for _, statement := range statements {
		grants = append(grants, model.PermissionGrant{
			ID:          statement.ID,
			Description: statement.Description,
			Granted:     statement.Granted,
		})
	}
	return &Viewer{
		path: resolved,
		summary: Summary{
			FileName:         filepath.Base(resolved),
			PackageSize:      info.Size(),
			UncompressedSize: uncompressed,
			SHA256:           checksum,
			Manifest:         manifest,
			Compatibility:    recipe.CurrentPlatformCompatible(manifest),
			Permissions:      grants,
			Files:            files,
		},
		files: byName,
	}, nil
}

func (v *Viewer) Summary() Summary {
	return v.summary
}

func (v *Viewer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", v.index)
	mux.HandleFunc("/api/package", v.packageInfo)
	mux.HandleFunc("/api/file", v.previewFile)
	mux.HandleFunc("/api/heartbeat", heartbeat)
	return viewerSecurityHeaders(mux)
}

func (v *Viewer) Start() (*Running, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	running := &Running{
		URL:    "http://" + listener.Addr().String(),
		server: &http.Server{Handler: nil, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second},
		done:   make(chan error, 1),
	}
	running.lastActivity.Store(time.Now().UnixNano())
	handler := v.Handler()
	running.server.Handler = http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		running.lastActivity.Store(time.Now().UnixNano())
		handler.ServeHTTP(response, request)
	})
	go func() {
		err := running.server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		running.done <- err
		close(running.done)
	}()
	return running, nil
}

func (r *Running) Wait(ctx context.Context) error {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-r.done:
			return err
		case <-ctx.Done():
			return r.shutdown()
		case <-ticker.C:
			last := time.Unix(0, r.lastActivity.Load())
			if time.Since(last) >= idleTimeout {
				return r.shutdown()
			}
		}
	}
}

func (r *Running) Close() error {
	return r.shutdown()
}

func (r *Running) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := r.server.Shutdown(ctx); err != nil {
		return err
	}
	if err, ok := <-r.done; ok {
		return err
	}
	return nil
}

func (v *Viewer) index(response http.ResponseWriter, request *http.Request) {
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
	_, _ = io.WriteString(response, viewerPage)
}

func (v *Viewer) packageInfo(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", "GET")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(v.summary)
}

func (v *Viewer) previewFile(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	file, ok := v.files[request.URL.Query().Get("name")]
	if !ok {
		writeViewerError(response, http.StatusNotFound, "Package file not found.")
		return
	}
	if file.Preview == "none" {
		writeViewerError(response, http.StatusUnsupportedMediaType, "This file type cannot be previewed safely.")
		return
	}
	if file.Size > maxPreviewSize {
		writeViewerError(response, http.StatusRequestEntityTooLarge, "Files larger than 2 MB are not previewed.")
		return
	}
	data, err := artifacts.ReadFile(v.path, file.Name)
	if err != nil {
		writeViewerError(response, http.StatusInternalServerError, "Unable to read this package file.")
		return
	}
	if file.Preview == "text" && !utf8.Valid(data) {
		writeViewerError(response, http.StatusUnsupportedMediaType, "This file contains binary data and cannot be previewed as text.")
		return
	}
	response.Header().Set("Content-Type", file.MediaType)
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Length", fmt.Sprint(len(data)))
	if request.Method == http.MethodHead {
		return
	}
	_, _ = response.Write(data)
}

func heartbeat(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusNoContent)
}

func previewType(name string, size uint64) (string, string) {
	if size > maxPreviewSize {
		return "none", ""
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image", "image/png"
	case ".jpg", ".jpeg":
		return "image", "image/jpeg"
	case ".gif":
		return "image", "image/gif"
	case ".webp":
		return "image", "image/webp"
	case ".yml", ".yaml", ".json", ".md", ".txt", ".go", ".js", ".jsx", ".ts",
		".tsx", ".css", ".html", ".htm", ".xml", ".toml", ".ini", ".sh", ".ps1",
		".svg", ".csv":
		return "text", "text/plain; charset=utf-8"
	default:
		return "none", ""
	}
}

func writeViewerError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]any{
		"error": map[string]string{"message": message},
	})
}

func viewerSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(response, request)
	})
}
