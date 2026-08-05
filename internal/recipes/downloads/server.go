package downloads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

const (
	stateQueued      = "queued"
	stateDownloading = "downloading"
	statePaused      = "paused"
	stateCompleted   = "completed"
	stateFailed      = "failed"

	maxQueuedDownloads    = 100
	maxDownloadSize       = int64(20 * 1024 * 1024 * 1024)
	minimumStorageReserve = uint64(512 * 1024 * 1024)
	downloadIdleTimeout   = 30 * time.Second
)

type item struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Name         string    `json:"name"`
	FinalPath    string    `json:"finalPath,omitempty"`
	PartialPath  string    `json:"partialPath,omitempty"`
	State        string    `json:"state"`
	Downloaded   int64     `json:"downloaded"`
	Total        int64     `json:"total,omitempty"`
	Speed        int64     `json:"speed,omitempty"`
	Error        string    `json:"error,omitempty"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"lastModified,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type server struct {
	mu          sync.Mutex
	destination string
	root        *os.Root
	statePath   string
	items       []item
	cancels     map[string]context.CancelFunc
	gate        *pairing.Gate
	client      *http.Client
	wake        chan struct{}
}

func newServer(values map[string]any, dataPath string) (*server, error) {
	destination, _ := values["destination"].(string)
	code, _ := values["pairing-code"].(string)
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dataPath, 0o700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return nil, err
	}
	gate, err := pairing.New(code, "Downloads", dataPath)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	value := &server{
		destination: destination,
		root:        root,
		statePath:   filepath.Join(dataPath, "downloads.json"),
		cancels:     map[string]context.CancelFunc{},
		gate:        gate,
		wake:        make(chan struct{}, 1),
		client:      newDownloadHTTPClient(),
	}
	if err := value.load(); err != nil {
		_ = root.Close()
		return nil, err
	}
	value.mu.Lock()
	for index := range value.items {
		if value.items[index].State == stateDownloading {
			value.items[index].State = stateQueued
		}
	}
	_ = value.saveLocked()
	value.mu.Unlock()
	return value, nil
}

func (s *server) serve(port, healthPort int) error {
	defer s.root.Close()
	healthServer, err := health.Start(healthPort, func() health.Snapshot {
		info, statErr := os.Stat(s.destination)
		if statErr != nil || !info.IsDir() {
			return health.Snapshot{
				Status:          "degraded",
				ProblemCode:     "download_folder_unavailable",
				ProblemSummary:  "The Downloads folder is unavailable.",
				ProblemRecovery: "Restore or reconnect the selected folder. Downloads will resume automatically.",
			}
		}
		return health.Snapshot{Status: "healthy"}
	})
	if err != nil {
		return err
	}
	defer healthServer.Close()
	go s.runQueue()
	s.signal()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/downloads", s.add)
	mux.HandleFunc("/downloads/", s.action)
	mux.HandleFunc("/files/", s.file)
	mux.HandleFunc("/open/", s.openCompleted)
	handler := downloadSecurityHeaders(s.gate.Middleware(mux))
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

func newDownloadHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, errors.New("download address is invalid")
			}
			addresses, err := publicDownloadAddresses(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, candidate := range addresses {
				connection, dialErr := dialer.DialContext(
					ctx,
					network,
					net.JoinHostPort(candidate.String(), port),
				)
				if dialErr == nil {
					return &idleTimeoutConnection{
						Conn:    connection,
						timeout: downloadIdleTimeout,
					}, nil
				}
				lastErr = dialErr
			}
			if lastErr == nil {
				lastErr = errors.New("download address has no public IP address")
			}
			return nil, lastErr
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("download followed too many redirects")
			}
			if _, err := validateURL(request.URL.String()); err != nil {
				return err
			}
			if len(via) > 0 &&
				via[len(via)-1].URL.Scheme == "https" &&
				request.URL.Scheme != "https" {
				return errors.New("download refused an insecure HTTPS redirect")
			}
			return nil
		},
	}
}

type idleTimeoutConnection struct {
	net.Conn
	timeout time.Duration
}

func (c *idleTimeoutConnection) Read(buffer []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(buffer)
}

func (c *idleTimeoutConnection) Write(buffer []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(buffer)
}

func publicDownloadAddresses(ctx context.Context, host string) ([]netip.Addr, error) {
	if parsed, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		parsed = parsed.Unmap()
		if !publicDownloadAddress(parsed) {
			return nil, errors.New("Downloads cannot access local, private, or special-use addresses")
		}
		return []netip.Addr{parsed}, nil
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve download address: %w", err)
	}
	if len(addresses) == 0 {
		return nil, errors.New("download address has no IP address")
	}
	result := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if !publicDownloadAddress(address) {
			return nil, errors.New("Downloads cannot access hostnames that resolve to local, private, or special-use addresses")
		}
		result = append(result, address)
	}
	return result, nil
}

func publicDownloadAddress(address netip.Addr) bool {
	if !address.IsValid() ||
		!address.IsGlobalUnicast() ||
		address.IsPrivate() ||
		address.IsLoopback() ||
		address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() ||
		address.IsMulticast() ||
		address.IsUnspecified() {
		return false
	}
	for _, blocked := range blockedDownloadNetworks {
		if blocked.Contains(address) {
			return false
		}
	}
	return true
}

var blockedDownloadNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func (s *server) home(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", "GET")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	items := append([]item(nil), s.items...)
	s.mu.Unlock()
	sort.SliceStable(items, func(left, right int) bool {
		return items[left].CreatedAt.After(items[right].CreatedAt)
	})
	data := struct {
		Styles template.CSS
		Items  []item
		Local  bool
	}{
		Styles: template.CSS(webui.Styles),
		Items:  items,
		Local:  localDownloadRequest(request),
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if err := downloadsTemplate.Execute(response, data); err != nil {
		http.Error(response, "Unable to show Downloads.", http.StatusInternalServerError)
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
		http.Error(response, "Unable to read this download.", http.StatusBadRequest)
		return
	}
	address, err := validateURL(request.FormValue("url"))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if len(s.items) >= maxQueuedDownloads {
		s.mu.Unlock()
		http.Error(response, "Downloads can queue no more than 100 items.", http.StatusConflict)
		return
	}
	s.mu.Unlock()
	id, err := downloadID()
	if err != nil {
		http.Error(response, "Unable to create this download.", http.StatusInternalServerError)
		return
	}
	name := safeName(filepath.Base(address.Path))
	if name == "" {
		name = "download"
	}
	now := time.Now().UTC()
	partial := ".spare-" + id + ".part"
	s.mu.Lock()
	if len(s.items) >= maxQueuedDownloads {
		s.mu.Unlock()
		http.Error(response, "Downloads can queue no more than 100 items.", http.StatusConflict)
		return
	}
	s.items = append(s.items, item{
		ID:          id,
		URL:         address.String(),
		Name:        name,
		PartialPath: partial,
		State:       stateQueued,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	err = s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		http.Error(response, "Unable to save this download.", http.StatusInternalServerError)
		return
	}
	s.signal()
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) action(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/downloads/"), "/")
	if len(parts) != 2 {
		http.NotFound(response, request)
		return
	}
	id, action := parts[0], parts[1]
	s.mu.Lock()
	index := s.indexLocked(id)
	if index < 0 {
		s.mu.Unlock()
		http.NotFound(response, request)
		return
	}
	current := &s.items[index]
	switch action {
	case "pause":
		if current.State == stateDownloading || current.State == stateQueued {
			current.State = statePaused
			current.UpdatedAt = time.Now().UTC()
			if cancel := s.cancels[id]; cancel != nil {
				cancel()
			}
		}
	case "resume", "retry":
		if current.State == statePaused || current.State == stateFailed {
			current.State = stateQueued
			current.Error = ""
			current.UpdatedAt = time.Now().UTC()
		}
	case "cancel":
		if cancel := s.cancels[id]; cancel != nil {
			cancel()
		}
		if partial, pathErr := s.destinationName(current.PartialPath); pathErr == nil && partial != "" {
			_ = s.root.Remove(partial)
		}
		s.items = append(s.items[:index], s.items[index+1:]...)
	default:
		s.mu.Unlock()
		http.NotFound(response, request)
		return
	}
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		http.Error(response, "Unable to update this download.", http.StatusInternalServerError)
		return
	}
	s.signal()
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func (s *server) runQueue() {
	for range s.wake {
		for {
			s.mu.Lock()
			index := -1
			for candidate := range s.items {
				if s.items[candidate].State == stateQueued {
					index = candidate
					break
				}
			}
			if index < 0 {
				s.mu.Unlock()
				break
			}
			id := s.items[index].ID
			s.items[index].State = stateDownloading
			s.items[index].Error = ""
			s.items[index].UpdatedAt = time.Now().UTC()
			_ = s.saveLocked()
			s.mu.Unlock()
			s.download(id)
		}
	}
}

func (s *server) download(id string) {
	s.mu.Lock()
	index := s.indexLocked(id)
	if index < 0 {
		s.mu.Unlock()
		return
	}
	current := s.items[index]
	ctx, cancel := context.WithCancel(context.Background())
	s.cancels[id] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.cancels, id)
		s.mu.Unlock()
	}()

	partialName, err := s.destinationName(current.PartialPath)
	if err != nil || partialName == "" {
		s.fail(id, errors.New("download partial file is outside the selected folder"))
		return
	}
	existing := int64(0)
	partialExists := false
	if info, statErr := s.root.Lstat(partialName); statErr == nil {
		if !info.Mode().IsRegular() {
			s.fail(id, errors.New("download partial file is not a regular file"))
			return
		}
		existing = info.Size()
		partialExists = true
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		s.fail(id, statErr)
		return
	}
	if existing > maxDownloadSize {
		s.fail(id, fmt.Errorf("partial download exceeds the %s file limit", byteLabel(maxDownloadSize)))
		return
	}
	validator := resumeValidator(current)
	resuming := existing > 0 && validator != ""
	if existing > 0 && !resuming {
		existing = 0
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, current.URL, nil)
	if err != nil {
		s.fail(id, err)
		return
	}
	request.Header.Set("User-Agent", "Spare Downloads/0.1")
	if resuming {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
		request.Header.Set("If-Range", validator)
	}
	response, err := s.client.Do(request)
	if err != nil {
		s.failUnlessPaused(id, safeDownloadError(err))
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		s.fail(id, fmt.Errorf("server returned HTTP %d", response.StatusCode))
		return
	}

	start := int64(0)
	if response.StatusCode == http.StatusPartialContent {
		expected := int64(0)
		if resuming {
			expected = existing
		}
		rangeStart, ok := contentRangeStart(response.Header.Get("Content-Range"))
		if !ok || rangeStart != expected {
			s.fail(id, errors.New("server returned an invalid resume range"))
			return
		}
		start = expected
	}
	if response.ContentLength > maxDownloadSize-start {
		s.fail(id, fmt.Errorf("download exceeds the %s file limit", byteLabel(maxDownloadSize)))
		return
	}
	if err := ensureDownloadCapacity(s.destination, response.ContentLength); err != nil {
		s.fail(id, err)
		return
	}

	flags := os.O_WRONLY
	if start > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		if !partialExists {
			flags |= os.O_CREATE | os.O_EXCL
		}
	}
	output, err := s.root.OpenFile(partialName, flags, 0o600)
	if err != nil {
		s.fail(id, err)
		return
	}
	outputClosed := false
	defer func() {
		if !outputClosed {
			_ = output.Close()
		}
	}()
	if err := output.Chmod(0o600); err != nil {
		s.fail(id, err)
		return
	}

	name := responseName(response, current.Name)
	total := int64(0)
	if response.ContentLength >= 0 {
		total = response.ContentLength
		total += start
	}
	etag := strings.TrimSpace(response.Header.Get("ETag"))
	lastModified := strings.TrimSpace(response.Header.Get("Last-Modified"))
	s.progress(id, name, start, total, 0, etag, lastModified)
	buffer := make([]byte, 64*1024)
	downloaded := start
	windowBytes := int64(0)
	windowStarted := time.Now()
	lastUpdate := time.Now()
	lastCapacityCheck := downloaded
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if downloaded > maxDownloadSize-int64(count) {
				s.fail(id, fmt.Errorf("download exceeds the %s file limit", byteLabel(maxDownloadSize)))
				return
			}
			if downloaded-lastCapacityCheck >= 8*1024*1024 {
				if err := ensureDownloadCapacity(s.destination, int64(count)); err != nil {
					s.fail(id, err)
					return
				}
				lastCapacityCheck = downloaded
			}
			written, writeErr := output.Write(buffer[:count])
			if writeErr != nil {
				s.fail(id, writeErr)
				return
			}
			if written != count {
				s.fail(id, io.ErrShortWrite)
				return
			}
			downloaded += int64(written)
			windowBytes += int64(written)
		}
		if time.Since(lastUpdate) >= 400*time.Millisecond || readErr != nil {
			elapsed := time.Since(windowStarted).Seconds()
			speed := int64(0)
			if elapsed > 0 {
				speed = int64(float64(windowBytes) / elapsed)
			}
			s.progress(id, name, downloaded, total, speed, etag, lastModified)
			lastUpdate = time.Now()
			if elapsed >= 2 {
				windowStarted = time.Now()
				windowBytes = 0
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			s.failUnlessPaused(id, readErr)
			return
		}
	}
	if err := output.Sync(); err != nil {
		s.fail(id, err)
		return
	}
	if err := output.Close(); err != nil {
		s.fail(id, err)
		return
	}
	outputClosed = true
	finalPath, err := s.finalizeDownload(partialName, name)
	if err != nil {
		s.fail(id, err)
		return
	}
	s.mu.Lock()
	index = s.indexLocked(id)
	if index >= 0 {
		s.items[index].Name = filepath.Base(finalPath)
		s.items[index].FinalPath = finalPath
		s.items[index].PartialPath = ""
		s.items[index].State = stateCompleted
		s.items[index].Downloaded = downloaded
		s.items[index].Total = downloaded
		s.items[index].Speed = 0
		s.items[index].Error = ""
		s.items[index].UpdatedAt = time.Now().UTC()
		_ = s.saveLocked()
	}
	s.mu.Unlock()
}

func (s *server) progress(
	id,
	name string,
	downloaded,
	total,
	speed int64,
	etag,
	lastModified string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.indexLocked(id)
	if index < 0 || s.items[index].State != stateDownloading {
		return
	}
	s.items[index].Name = name
	s.items[index].Downloaded = downloaded
	s.items[index].Total = total
	s.items[index].Speed = speed
	s.items[index].ETag = etag
	s.items[index].LastModified = lastModified
	s.items[index].UpdatedAt = time.Now().UTC()
	_ = s.saveLocked()
}

func (s *server) failUnlessPaused(id string, err error) {
	s.mu.Lock()
	index := s.indexLocked(id)
	paused := index < 0 || s.items[index].State == statePaused
	s.mu.Unlock()
	if !paused {
		s.fail(id, err)
	}
}

func (s *server) fail(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.indexLocked(id)
	if index < 0 || s.items[index].State == statePaused {
		return
	}
	s.items[index].State = stateFailed
	s.items[index].Speed = 0
	s.items[index].Error = err.Error()
	s.items[index].UpdatedAt = time.Now().UTC()
	_ = s.saveLocked()
}

func (s *server) destinationName(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	name := value
	if filepath.IsAbs(name) {
		relative, err := filepath.Rel(s.destination, name)
		if err != nil {
			return "", err
		}
		name = relative
	}
	name = filepath.Clean(name)
	if name == "." ||
		name == ".." ||
		filepath.Base(name) != name ||
		strings.ContainsAny(name, `/\`) {
		return "", errors.New("download file is outside the selected folder")
	}
	return name, nil
}

func (s *server) finalizeDownload(partialName, requestedName string) (string, error) {
	info, err := s.root.Lstat(partialName)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("download partial file is not a regular file")
	}
	name := safeName(requestedName)
	if name == "" {
		name = "download"
	}
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	for index := 0; index < 10_000; index++ {
		candidate := name
		if index > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", base, index, extension)
		}
		if err := s.root.Link(partialName, candidate); err == nil {
			if removeErr := s.root.Remove(partialName); removeErr != nil {
				_ = s.root.Remove(candidate)
				return "", removeErr
			}
			return filepath.Join(s.destination, candidate), nil
		} else if errors.Is(err, fs.ErrExist) {
			continue
		}

		// Some removable and network filesystems do not support hard links.
		// O_EXCL keeps the copy fallback from replacing an existing file.
		if capacityErr := ensureDownloadCapacity(s.destination, info.Size()); capacityErr != nil {
			return "", capacityErr
		}
		input, openErr := s.root.Open(partialName)
		if openErr != nil {
			return "", openErr
		}
		inputInfo, statErr := input.Stat()
		if statErr != nil || !inputInfo.Mode().IsRegular() || !os.SameFile(info, inputInfo) {
			_ = input.Close()
			if statErr != nil {
				return "", statErr
			}
			return "", errors.New("download partial file changed before completion")
		}
		output, createErr := s.root.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(createErr, fs.ErrExist) {
			_ = input.Close()
			continue
		}
		if createErr != nil {
			_ = input.Close()
			return "", createErr
		}
		_, copyErr := io.Copy(output, input)
		syncErr := output.Sync()
		closeOutputErr := output.Close()
		closeInputErr := input.Close()
		if copyErr != nil || syncErr != nil || closeOutputErr != nil || closeInputErr != nil {
			_ = s.root.Remove(candidate)
			for _, candidateErr := range []error{copyErr, syncErr, closeOutputErr, closeInputErr} {
				if candidateErr != nil {
					return "", candidateErr
				}
			}
		}
		if removeErr := s.root.Remove(partialName); removeErr != nil {
			_ = s.root.Remove(candidate)
			return "", removeErr
		}
		return filepath.Join(s.destination, candidate), nil
	}
	return "", errors.New("unable to choose a unique download filename")
}

func ensureDownloadCapacity(destination string, incoming int64) error {
	available, err := availableDownloadBytes(destination)
	if err != nil {
		return fmt.Errorf("check available download storage: %w", err)
	}
	if available <= minimumStorageReserve {
		return fmt.Errorf(
			"Downloads stopped to preserve %s of free storage",
			byteLabel(int64(minimumStorageReserve)),
		)
	}
	if incoming > 0 && uint64(incoming) > available-minimumStorageReserve {
		return errors.New("this download does not fit while preserving free storage")
	}
	return nil
}

func resumeValidator(value item) string {
	etag := strings.TrimSpace(value.ETag)
	if etag != "" && !strings.HasPrefix(strings.ToUpper(etag), "W/") {
		return etag
	}
	return strings.TrimSpace(value.LastModified)
}

func contentRangeStart(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "bytes ") {
		return 0, false
	}
	value = strings.TrimSpace(value[len("bytes "):])
	slash := strings.IndexByte(value, '/')
	if slash < 0 {
		return 0, false
	}
	byteRange := value[:slash]
	dash := strings.IndexByte(byteRange, '-')
	if dash <= 0 {
		return 0, false
	}
	start, err := strconv.ParseInt(byteRange[:dash], 10, 64)
	return start, err == nil && start >= 0
}

func safeDownloadError(err error) error {
	var requestError *url.Error
	if errors.As(err, &requestError) && requestError.Err != nil {
		return errors.New(requestError.Err.Error())
	}
	return err
}

func (s *server) file(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/files/")
	s.mu.Lock()
	index := s.indexLocked(id)
	if index < 0 || s.items[index].State != stateCompleted {
		s.mu.Unlock()
		http.NotFound(response, request)
		return
	}
	selected := s.items[index]
	s.mu.Unlock()
	name, err := s.destinationName(selected.FinalPath)
	if err != nil || name == "" {
		http.NotFound(response, request)
		return
	}
	before, err := s.root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() {
		http.NotFound(response, request)
		return
	}
	file, err := s.root.Open(name)
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
	response.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, strings.ReplaceAll(selected.Name, `"`, "")))
	http.ServeContent(response, request, selected.Name, info.ModTime(), file)
}

func (s *server) openCompleted(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !localDownloadRequest(request) {
		http.Error(response, "Completed files can only be opened from Spare on this computer.", http.StatusForbidden)
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/open/")
	s.mu.Lock()
	index := s.indexLocked(id)
	if index < 0 || s.items[index].State != stateCompleted {
		s.mu.Unlock()
		http.NotFound(response, request)
		return
	}
	selected := s.items[index]
	s.mu.Unlock()
	name, err := s.destinationName(selected.FinalPath)
	if err != nil || name == "" {
		http.NotFound(response, request)
		return
	}
	info, err := s.root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(response, request)
		return
	}
	path := filepath.Join(s.destination, name)
	if err := revealDownloadedFile(path); err != nil {
		http.Error(response, "Unable to show the completed file.", http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func localDownloadRequest(request *http.Request) bool {
	host := request.RemoteAddr
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	address := net.ParseIP(strings.Trim(host, "[]"))
	return address != nil && address.IsLoopback()
}

var revealDownloadedFile = func(path string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", "-R", path)
	case "windows":
		command = exec.Command("explorer.exe", "/select,"+path)
	default:
		command = exec.Command("xdg-open", filepath.Dir(path))
	}
	return command.Run()
}

func (s *server) indexLocked(id string) int {
	for index := range s.items {
		if s.items[index].ID == id {
			return index
		}
	}
	return -1
}

func (s *server) load() error {
	data, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.items)
}

func (s *server) saveLocked() error {
	data, err := json.MarshalIndent(s.items, "", "  ")
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
	case s.wake <- struct{}{}:
	default:
	}
}

func validateURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return nil, errors.New("enter a complete HTTP or HTTPS URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("Downloads supports only HTTP and HTTPS URLs")
	}
	if parsed.User != nil {
		return nil, errors.New("URLs containing usernames or passwords are not supported")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, errors.New("Downloads cannot access this computer through a download URL")
	}
	if address, parseErr := netip.ParseAddr(strings.Trim(host, "[]")); parseErr == nil &&
		!publicDownloadAddress(address.Unmap()) {
		return nil, errors.New("Downloads cannot access local, private, or special-use addresses")
	}
	return parsed, nil
}

func responseName(response *http.Response, fallback string) string {
	if disposition := response.Header.Get("Content-Disposition"); disposition != "" {
		if _, parameters, err := mime.ParseMediaType(disposition); err == nil {
			if name := safeName(filepath.Base(parameters["filename"])); name != "" {
				return name
			}
		}
	}
	if name := safeName(filepath.Base(response.Request.URL.Path)); name != "" {
		return name
	}
	return fallback
}

func safeName(value string) string {
	value = strings.Map(func(character rune) rune {
		if character == 0 || character < 32 || character == 127 {
			return -1
		}
		return character
	}, value)
	value = strings.TrimSpace(value)
	value = filepath.Base(value)
	if value == "." || value == string(filepath.Separator) {
		return ""
	}
	value = strings.TrimRight(value, ". ")
	runes := []rune(value)
	if len(runes) > 180 {
		value = string(runes[:180])
	}
	stem := strings.ToUpper(strings.TrimSuffix(value, filepath.Ext(value)))
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" ||
		(len(stem) == 4 &&
			(strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) &&
			stem[3] >= '1' && stem[3] <= '9') {
		value = "_" + value
	}
	return value
}

func downloadID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func downloadSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'self' wails://wails http://127.0.0.1:* http://localhost:*; base-uri 'none'")
		next.ServeHTTP(response, request)
	})
}

func byteLabel(value int64) string {
	if value >= 1_000_000_000 {
		return fmt.Sprintf("%.1f GB", float64(value)/1_000_000_000)
	}
	if value >= 1_000_000 {
		return fmt.Sprintf("%.1f MB", float64(value)/1_000_000)
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1f KB", float64(value)/1000)
	}
	return fmt.Sprintf("%d B", value)
}

var downloadsTemplate = template.Must(template.New("downloads").Funcs(template.FuncMap{
	"bytes": byteLabel,
	"percent": func(downloaded, total int64) int64 {
		if total <= 0 {
			return 0
		}
		value := downloaded * 100 / total
		if value > 100 {
			return 100
		}
		return value
	},
	"canPause": func(state string) bool {
		return state == stateQueued || state == stateDownloading
	},
	"canResume": func(state string) bool {
		return state == statePaused
	},
	"canRetry": func(state string) bool {
		return state == stateFailed
	},
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Downloads</title><style>{{.Styles}}
.download-layout{grid-template-columns:minmax(15rem,.72fr) minmax(0,1.28fr);align-items:start}.download-form{gap:10px}.download-url-row{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:end;gap:8px}.download-url-row .field{min-width:0}.download-url-row input{min-height:36px}.download-url-row button,.download-item button,.download-item .button{min-height:32px;padding-inline:10px;font-size:13px}.queue-card{gap:8px}.download-item{gap:8px;padding:10px;border-radius:8px;background:#292929;box-shadow:inset 0 .1px .2px .1px #ffffff33,0 0 0 1px #ffffff08}.download-item .actions{gap:6px}.download-item .actions form{display:block}.download-item .row{gap:10px}.download-item .meta{font-weight:400}.download-item .progress{height:5px}.download-item h2{line-height:1.35}@media(max-width:760px){.download-layout{grid-template-columns:1fr}.download-url-row{grid-template-columns:1fr}.download-url-row button{width:100%}}
</style></head>
<body><main class="shell"><header><div><p class="eyebrow">Download box</p><h1>Downloads</h1><p class="subtitle">Send an HTTP or HTTPS link from a paired device. Spare downloads one file at a time and keeps the rest queued.</p></div><form method="post" action="/pair/revoke"><button class="secondary" type="submit">Revoke devices</button></form></header>
<section class="grid download-layout"><form class="card download-form" method="post" action="/downloads"><h2>Add a download</h2><div class="download-url-row"><label class="field" for="url">File URL<input id="url" name="url" type="url" inputmode="url" required placeholder="https://example.com/file.zip"></label><button type="submit">Add to queue</button></div></form>
<section class="card stack queue-card" aria-labelledby="queue-heading"><h2 id="queue-heading">Queue</h2>{{if .Items}}{{range .Items}}<article class="download-item stack"><div class="row"><div><h2 style="overflow-wrap:anywhere">{{.Name}}</h2><p class="meta">{{.State}}{{if .Speed}} · {{bytes .Speed}}/s{{end}}</p></div>{{if .Total}}<span class="meta">{{percent .Downloaded .Total}}%</span>{{end}}</div>{{if .Total}}<div class="progress" role="progressbar" aria-label="{{.Name}}" aria-valuenow="{{percent .Downloaded .Total}}" aria-valuemin="0" aria-valuemax="100"><span style="width:{{percent .Downloaded .Total}}%"></span></div>{{end}}<p class="meta">{{bytes .Downloaded}}{{if .Total}} of {{bytes .Total}}{{end}}</p>{{if .Error}}<p class="danger" role="alert">{{.Error}}</p>{{end}}<div class="actions">{{if eq .State "completed"}}{{if $.Local}}<form method="post" action="/open/{{.ID}}"><button type="submit">Show completed file</button></form>{{else}}<a class="button" href="/files/{{.ID}}">Download file</a>{{end}}{{end}}{{if canPause .State}}<form method="post" action="/downloads/{{.ID}}/pause"><button class="secondary" type="submit">Pause download</button></form>{{end}}{{if canResume .State}}<form method="post" action="/downloads/{{.ID}}/resume"><button type="submit">Resume download</button></form>{{end}}{{if canRetry .State}}<form method="post" action="/downloads/{{.ID}}/retry"><button type="submit">Retry download</button></form>{{end}}<form method="post" action="/downloads/{{.ID}}/cancel"><button class="secondary" type="submit">Remove download</button></form></div></article>{{end}}{{else}}<div class="empty"><h2>Nothing queued</h2><p>Add a direct file link and Spare will download it in the background.</p></div>{{end}}</section></section>
</main></body></html>`))
