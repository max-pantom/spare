package clipboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spare-run/spare/internal/config"
	"github.com/spare-run/spare/internal/health"
	"github.com/spare-run/spare/internal/profile"
	"github.com/spare-run/spare/internal/recipes/shared/pairing"
	"github.com/spare-run/spare/internal/recipes/shared/webui"
)

const (
	maxClipboardEntries       = 100
	maxClipboardStateBytes    = 8 * 1024 * 1024
	maxClipboardStorageBytes  = uint64(1024 * 1024 * 1024)
	maxClipboardTextBytes     = 1024 * 1024
	minimumClipboardFreeBytes = uint64(256 * 1024 * 1024)
)

type entry struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text,omitempty"`
	Name      string    `json:"name,omitempty"`
	FileName  string    `json:"fileName,omitempty"`
	Size      int64     `json:"size,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type server struct {
	mu            sync.Mutex
	root          string
	files         string
	fileRoot      *os.Root
	statePath     string
	maximum       int64
	defaultExpiry int64
	entries       []entry
	gate          *pairing.Gate
	uploadSlot    chan struct{}
}

func newServer(values map[string]any, dataPath string) (*server, error) {
	code, _ := values["pairing-code"].(string)
	maximum, err := config.ParseSize(values["max-file-size"])
	if err != nil {
		return nil, err
	}
	expiry, err := integerValue(values["default-expiry"])
	if err != nil {
		return nil, err
	}
	if maximum <= 0 {
		maximum = 25_000_000
	}
	if expiry <= 0 {
		expiry = 60
	}
	root, err := filepath.Abs(dataPath)
	if err != nil {
		return nil, err
	}
	files := filepath.Join(root, "files")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(files, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, err
	}
	fileRoot, err := os.OpenRoot(files)
	if err != nil {
		return nil, err
	}
	gate, err := pairing.New(code, "Clipboard", root)
	if err != nil {
		_ = fileRoot.Close()
		return nil, err
	}
	value := &server{
		root:          root,
		files:         files,
		fileRoot:      fileRoot,
		statePath:     filepath.Join(root, "clipboard.json"),
		maximum:       maximum,
		defaultExpiry: expiry,
		gate:          gate,
		uploadSlot:    make(chan struct{}, 1),
	}
	if err := value.load(); err != nil {
		_ = fileRoot.Close()
		return nil, err
	}
	value.cleanup()
	return value, nil
}

func (s *server) serve(port, healthPort int) error {
	defer s.fileRoot.Close()
	healthServer, err := health.Start(healthPort, func() health.Snapshot {
		info, statErr := os.Stat(s.root)
		if statErr != nil || !info.IsDir() {
			return health.Snapshot{
				Status:          "degraded",
				ProblemCode:     "clipboard_storage_unavailable",
				ProblemSummary:  "Clipboard storage is unavailable.",
				ProblemRecovery: "Restart Clipboard after restoring Spare's application data folder.",
			}
		}
		return health.Snapshot{Status: "healthy"}
	})
	if err != nil {
		return err
	}
	defer healthServer.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/entries", s.add)
	mux.HandleFunc("/entries/", s.entryAction)
	mux.HandleFunc("/clear", s.clear)
	mux.HandleFunc("/files/", s.file)
	handler := securityHeaders(s.gate.Middleware(mux))

	stopCleanup := make(chan struct{})
	defer close(stopCleanup)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanup()
			case <-stopCleanup:
				return
			}
		}
	}()

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 * 1024,
	}
	return httpServer.ListenAndServe()
}

func (s *server) home(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", "GET")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.cleanup()
	s.mu.Lock()
	entries := append([]entry(nil), s.entries...)
	s.mu.Unlock()
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].CreatedAt.After(entries[right].CreatedAt)
	})
	data := struct {
		Styles        template.CSS
		Entries       []entry
		DefaultExpiry int64
		Maximum       string
	}{
		Styles:        template.CSS(webui.Styles),
		Entries:       entries,
		DefaultExpiry: s.defaultExpiry,
		Maximum:       formatBytes(s.maximum),
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if err := clipboardTemplate.Execute(response, data); err != nil {
		http.Error(response, "Unable to show Clipboard.", http.StatusInternalServerError)
	}
}

func (s *server) add(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	select {
	case s.uploadSlot <- struct{}{}:
		defer func() { <-s.uploadSlot }()
	default:
		http.Error(response, "Clipboard is already receiving another entry.", http.StatusTooManyRequests)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, s.maximum+128*1024)
	if profile.StorageAvailable(os.TempDir()) < uint64(s.maximum)+minimumClipboardFreeBytes {
		http.Error(response, "This computer needs more free storage before receiving that file.", http.StatusInsufficientStorage)
		return
	}
	if err := request.ParseMultipartForm(1024 * 1024); err != nil {
		http.Error(response, "Choose a smaller file or shorter text entry.", http.StatusBadRequest)
		return
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	expiry := s.defaultExpiry
	if value, err := strconv.ParseInt(request.FormValue("expiry"), 10, 64); err == nil && value >= 5 && value <= 1440 {
		expiry = value
	}
	text := strings.TrimSpace(request.FormValue("text"))
	if len([]byte(text)) > 64*1024 {
		http.Error(response, "Text entries can contain up to 64 KB.", http.StatusBadRequest)
		return
	}
	file, header, fileErr := request.FormFile("file")
	entryCount := 0
	if fileErr == nil {
		entryCount++
	} else if !errors.Is(fileErr, http.ErrMissingFile) {
		http.Error(response, "Unable to read the selected file.", http.StatusBadRequest)
		return
	}
	if text != "" {
		entryCount++
	}
	if entryCount == 0 {
		http.Error(response, "Add text, a link, or a small file.", http.StatusBadRequest)
		return
	}
	if err := s.ensureEntryCapacity(entryCount); err != nil {
		if file != nil {
			_ = file.Close()
		}
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	if text != "" {
		if err := s.ensureTextCapacity(text); err != nil {
			if file != nil {
				_ = file.Close()
			}
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
	}
	if fileErr == nil {
		defer file.Close()
		_, err := s.addFile(file, header, expiry)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		if text != "" {
			if err := s.addText(text, expiry); err != nil {
				http.Error(response, err.Error(), http.StatusBadRequest)
				return
			}
		}
	} else if text != "" {
		if err := s.addText(text, expiry); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
	}
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) addText(text string, expiry int64) error {
	if len([]byte(text)) > 64*1024 {
		return errors.New("text entries can contain up to 64 KB")
	}
	kind := "text"
	if parsed, err := url.Parse(text); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		kind = "link"
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	if len(s.entries) >= maxClipboardEntries {
		s.mu.Unlock()
		return fmt.Errorf("Clipboard can hold no more than %d entries", maxClipboardEntries)
	}
	if !s.textFitsLocked(text) {
		s.mu.Unlock()
		return errors.New("Clipboard text storage is full; delete an entry and try again")
	}
	previousLength := len(s.entries)
	s.entries = append(s.entries, entry{
		ID:        id,
		Kind:      kind,
		Text:      text,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(expiry) * time.Minute),
	})
	err = s.saveLocked()
	if err != nil {
		s.entries = s.entries[:previousLength]
	}
	s.mu.Unlock()
	return err
}

func (s *server) addFile(file multipart.File, header *multipart.FileHeader, expiry int64) (entry, error) {
	name := safeClipboardName(header.Filename)
	if name == "" {
		return entry{}, errors.New("choose a named file")
	}
	id, err := randomID()
	if err != nil {
		return entry{}, err
	}
	storedName := id + filepath.Ext(name)
	limit, err := s.availableFileCapacity()
	if err != nil {
		return entry{}, err
	}
	if limit <= 0 {
		return entry{}, errors.New("Clipboard needs more free storage before receiving another file")
	}
	output, err := s.fileRoot.OpenFile(storedName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return entry{}, err
	}
	written, copyErr := io.Copy(output, io.LimitReader(file, limit+1))
	closeErr := output.Close()
	if copyErr != nil {
		_ = s.fileRoot.Remove(storedName)
		return entry{}, copyErr
	}
	if closeErr != nil {
		_ = s.fileRoot.Remove(storedName)
		return entry{}, closeErr
	}
	if written > limit {
		_ = s.fileRoot.Remove(storedName)
		if limit < s.maximum {
			return entry{}, errors.New("Clipboard needs more free storage before receiving that file")
		}
		return entry{}, fmt.Errorf("files can be up to %s", formatBytes(s.maximum))
	}
	now := time.Now().UTC()
	added := entry{
		ID:        id,
		Kind:      "file",
		Name:      name,
		FileName:  storedName,
		Size:      written,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(expiry) * time.Minute),
	}
	s.mu.Lock()
	if len(s.entries) >= maxClipboardEntries {
		s.mu.Unlock()
		_ = s.fileRoot.Remove(storedName)
		return entry{}, fmt.Errorf("Clipboard can hold no more than %d entries", maxClipboardEntries)
	}
	previousLength := len(s.entries)
	s.entries = append(s.entries, added)
	err = s.saveLocked()
	if err != nil {
		s.entries = s.entries[:previousLength]
	}
	s.mu.Unlock()
	if err != nil {
		_ = s.fileRoot.Remove(storedName)
		return entry{}, err
	}
	return added, nil
}

func (s *server) ensureEntryCapacity(count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if count <= 0 || len(s.entries)+count > maxClipboardEntries {
		return fmt.Errorf("Clipboard can hold no more than %d entries", maxClipboardEntries)
	}
	return nil
}

func (s *server) ensureTextCapacity(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.textFitsLocked(text) {
		return errors.New("Clipboard text storage is full; delete an entry and try again")
	}
	return nil
}

func (s *server) textFitsLocked(text string) bool {
	textBytes := len([]byte(text))
	for _, candidate := range s.entries {
		textBytes += len([]byte(candidate.Text))
	}
	return textBytes <= maxClipboardTextBytes
}

func (s *server) availableFileCapacity() (int64, error) {
	directory, err := s.fileRoot.Open(".")
	if err != nil {
		return 0, err
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return 0, readErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	var used uint64
	for _, candidate := range entries {
		info, err := candidate.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		size := uint64(info.Size())
		if size >= maxClipboardStorageBytes-used {
			used = maxClipboardStorageBytes
			break
		}
		used += size
	}
	if used >= maxClipboardStorageBytes {
		return 0, nil
	}
	available := profile.StorageAvailable(s.files)
	if available <= minimumClipboardFreeBytes {
		return 0, nil
	}
	capacity := maxClipboardStorageBytes - used
	if diskCapacity := available - minimumClipboardFreeBytes; diskCapacity < capacity {
		capacity = diskCapacity
	}
	if uint64(s.maximum) < capacity {
		capacity = uint64(s.maximum)
	}
	return int64(capacity), nil
}

func (s *server) entryAction(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !strings.HasSuffix(request.URL.Path, "/delete") {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/entries/"), "/delete")
	s.mu.Lock()
	for index, candidate := range s.entries {
		if candidate.ID != id {
			continue
		}
		if candidate.FileName != "" {
			_ = s.fileRoot.Remove(candidate.FileName)
		}
		s.entries = append(s.entries[:index], s.entries[index+1:]...)
		break
	}
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		http.Error(response, "Unable to delete this entry.", http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) clear(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	for _, candidate := range s.entries {
		if candidate.FileName != "" {
			_ = s.fileRoot.Remove(candidate.FileName)
		}
	}
	s.entries = nil
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		http.Error(response, "Unable to clear Clipboard.", http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) file(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/files/")
	s.mu.Lock()
	var selected *entry
	for index := range s.entries {
		if s.entries[index].ID == id && s.entries[index].Kind == "file" {
			copy := s.entries[index]
			selected = &copy
			break
		}
	}
	s.mu.Unlock()
	if selected == nil {
		http.NotFound(response, request)
		return
	}
	if !validStoredFileName(selected.FileName) {
		http.NotFound(response, request)
		return
	}
	before, err := s.fileRoot.Lstat(selected.FileName)
	if err != nil || !before.Mode().IsRegular() {
		http.NotFound(response, request)
		return
	}
	file, err := s.fileRoot.Open(selected.FileName)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.NotFound(response, request)
		return
	}
	if !info.Mode().IsRegular() || !os.SameFile(before, info) {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": selected.Name}))
	response.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(response, request, selected.Name, info.ModTime(), file)
}

func (s *server) cleanup() {
	now := time.Now().UTC()
	s.mu.Lock()
	kept := s.entries[:0]
	changed := false
	for _, candidate := range s.entries {
		if candidate.ExpiresAt.After(now) {
			kept = append(kept, candidate)
			continue
		}
		changed = true
		if candidate.FileName != "" {
			_ = s.fileRoot.Remove(candidate.FileName)
		}
	}
	s.entries = kept
	if changed {
		_ = s.saveLocked()
	}
	s.mu.Unlock()
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
	if info.Size() > maxClipboardStateBytes {
		return errors.New("Clipboard state is too large")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s.entries); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Clipboard state contains trailing data")
	}
	if len(s.entries) > maxClipboardEntries {
		return fmt.Errorf("Clipboard state contains more than %d entries", maxClipboardEntries)
	}
	seenIDs := make(map[string]bool, len(s.entries))
	seenFiles := make(map[string]bool, len(s.entries))
	textBytes := 0
	for _, candidate := range s.entries {
		if !validClipboardID(candidate.ID) || seenIDs[candidate.ID] {
			return errors.New("Clipboard state contains an invalid or duplicate entry ID")
		}
		seenIDs[candidate.ID] = true
		switch candidate.Kind {
		case "text", "link":
			if candidate.FileName != "" || len([]byte(candidate.Text)) > 64*1024 {
				return errors.New("Clipboard state contains an invalid text entry")
			}
			textBytes += len([]byte(candidate.Text))
		case "file":
			if candidate.Name == "" ||
				safeClipboardName(candidate.Name) != candidate.Name ||
				!validStoredFileName(candidate.FileName) ||
				candidate.FileName != candidate.ID+filepath.Ext(candidate.Name) ||
				candidate.Size < 0 ||
				seenFiles[candidate.FileName] {
				return errors.New("Clipboard state contains an invalid file entry")
			}
			seenFiles[candidate.FileName] = true
		default:
			return errors.New("Clipboard state contains an unsupported entry type")
		}
	}
	if textBytes > maxClipboardTextBytes {
		return errors.New("Clipboard state contains too much text")
	}
	return nil
}

func (s *server) saveLocked() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'self' wails://wails http://127.0.0.1:* http://localhost:*; base-uri 'none'")
		next.ServeHTTP(response, request)
	})
}

func integerValue(value any) (int64, error) {
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

func randomID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func safeClipboardName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f ||
			character == '/' || character == '\\' {
			return '_'
		}
		return character
	}, value)
	value = strings.Trim(value, " .")
	runes := []rune(value)
	if len(runes) > 180 {
		value = string(runes[:180])
	}
	return value
}

func validClipboardID(value string) bool {
	if len(value) != 24 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 12 && value == strings.ToLower(value)
}

func validStoredFileName(value string) bool {
	return value != "" &&
		len(value) <= 220 &&
		filepath.Base(value) == value &&
		!strings.ContainsAny(value, `/\`) &&
		!strings.HasPrefix(value, ".")
}

func formatBytes(value int64) string {
	if value >= 1_000_000 {
		return fmt.Sprintf("%.0f MB", float64(value)/1_000_000)
	}
	if value >= 1000 {
		return fmt.Sprintf("%.0f KB", float64(value)/1000)
	}
	return fmt.Sprintf("%d B", value)
}

var clipboardTemplate = template.Must(template.New("clipboard").Funcs(template.FuncMap{
	"remaining": func(value time.Time) string {
		duration := time.Until(value).Round(time.Minute)
		if duration < time.Minute {
			return "Less than a minute"
		}
		if duration >= time.Hour {
			return fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)
		}
		return fmt.Sprintf("%d min", int(duration.Minutes()))
	},
	"bytes": formatBytes,
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Clipboard</title><style>{{.Styles}}</style></head>
<body><main class="shell"><header><div><p class="eyebrow">Shared clipboard</p><h1>Available now</h1><p class="subtitle">Move text, links, and small files between paired devices. Entries remove themselves when they expire.</p></div><form method="post" action="/pair/revoke"><button class="secondary" type="submit">Revoke devices</button></form></header>
<section class="grid" aria-label="Add to Clipboard"><form class="card" method="post" action="/entries" enctype="multipart/form-data"><h2>Add something</h2><label class="field" for="text">Text or link<textarea id="text" name="text" placeholder="Paste text or a link"></textarea></label><label class="field" for="file">Small file<input id="file" name="file" type="file"><span class="meta">Up to {{.Maximum}}</span></label><label class="field" for="expiry">Remove after<select id="expiry" name="expiry"><option value="15">15 minutes</option><option value="{{.DefaultExpiry}}" selected>{{.DefaultExpiry}} minutes</option><option value="1440">24 hours</option></select></label><button type="submit">Add to Clipboard</button></form>
<section class="card stack" aria-labelledby="available-heading"><div class="row"><h2 id="available-heading">Entries</h2>{{if .Entries}}<form method="post" action="/clear"><button class="secondary" type="submit">Clear all</button></form>{{end}}</div>
{{if .Entries}}{{range .Entries}}<article class="card stack"><div class="row"><div><p class="eyebrow">{{.Kind}}</p>{{if eq .Kind "file"}}<h2>{{.Name}}</h2><p class="meta">{{bytes .Size}}</p>{{else}}<h2 style="overflow-wrap:anywhere">{{.Text}}</h2>{{end}}</div><span class="meta">{{remaining .ExpiresAt}}</span></div><div class="actions">{{if eq .Kind "file"}}<a class="button" href="/files/{{.ID}}">Download file</a>{{else if eq .Kind "link"}}<a class="button" href="{{.Text}}" rel="noreferrer">Open link</a>{{end}}<form method="post" action="/entries/{{.ID}}/delete"><button class="secondary" type="submit">Delete entry</button></form></div></article>{{end}}{{else}}<div class="empty"><h2>Clipboard is empty</h2><p>Add text, a link, or a small file to make it available on paired devices.</p></div>{{end}}</section></section>
</main></body></html>`))
