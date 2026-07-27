package site

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/config"
	"github.com/spare-run/spare/internal/health"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/recipe"
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

type Implementation struct{}

func New() *Implementation {
	return &Implementation{}
}

func (i *Implementation) Manifest() recipe.Manifest {
	return recipe.Manifest{
		Schema:      recipe.SchemaV1,
		ID:          "site",
		Name:        "Site",
		Version:     "0.1.0",
		Description: "Serve a folder as a read-only website on this computer and the local network.",
		Support: recipe.SupportSpec{
			Systems:       []string{"darwin", "windows", "linux"},
			Architectures: []string{"amd64", "arm64"},
		},
		Runtime: recipe.RuntimeSpec{Type: "native"},
		Resources: recipe.ResourceSpec{
			MemoryRecommendedBytes: 64 * 1024 * 1024,
			MemoryMaximumBytes:     256 * 1024 * 1024,
			CPUMaximum:             1,
			StorageMinimumBytes:    1024 * 1024,
		},
		Network: recipe.NetworkSpec{Visibility: "local", Port: "automatic"},
		Storage: recipe.StorageSpec{PathField: "path", ReadOnly: true},
		Health: recipe.HealthSpec{
			Type:             "http",
			Path:             "/",
			IntervalSeconds:  10,
			FailureThreshold: 3,
		},
		Config: map[string]config.Field{
			"path": {
				Type:        config.TypeDirectory,
				Label:       "Site folder",
				Description: "The folder Spare will serve read-only.",
				Required:    true,
			},
		},
		Permissions: permissions.Set{
			Filesystem:      permissions.Filesystem{Read: []string{"path"}},
			Network:         permissions.Network{Local: true, Internet: false},
			StartOnLogin:    true,
			RunInBackground: true,
		},
	}
}

func (i *Implementation) ResolveConfig(input map[string]any) (map[string]any, error) {
	resolved, err := config.Resolve(i.Manifest().Config, input)
	if err != nil {
		return nil, err
	}
	root, err := ValidateRoot(resolved["path"].(string))
	if err != nil {
		return nil, err
	}
	resolved["path"] = root
	return resolved, nil
}

func (i *Implementation) Serve(values map[string]any, port, healthPort int) error {
	root, ok := values["path"].(string)
	if !ok || root == "" {
		return errors.New("the Site worker is missing its folder")
	}
	handler, err := NewHandler(root)
	if err != nil {
		return err
	}
	healthServer, err := health.Start(healthPort, func() health.Snapshot {
		return siteHealth(root)
	})
	if err != nil {
		return err
	}
	defer healthServer.Close()

	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

func siteHealth(root string) health.Snapshot {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return health.Snapshot{
			Status:          "degraded",
			ProblemCode:     "selected_folder_unavailable",
			ProblemSummary:  "The selected folder is unavailable.",
			ProblemRecovery: "Reconnect or restore the folder. Site will recover automatically.",
		}
	}
	return health.Snapshot{Status: "healthy"}
}
