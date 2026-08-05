package drop

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/health"
)

type server struct {
	root            string
	maximumFileSize int64
	template        *template.Template
	uploadSlot      chan struct{}
}

type pageData struct {
	Files            []fileEntry
	AvailableStorage string
	MaximumFileSize  string
}

func newServer(root string, maximumFileSize int64) (*server, error) {
	page, err := template.New("drop").Funcs(template.FuncMap{
		"formatBytes": formatBytes,
	}).Parse(dropPage)
	if err != nil {
		return nil, err
	}
	return &server{
		root:            root,
		maximumFileSize: maximumFileSize,
		template:        page,
		uploadSlot:      make(chan struct{}, 1),
	}, nil
}

func (s *server) serve(port, healthPort int) error {
	healthServer, err := health.Start(healthPort, func() health.Snapshot {
		files, _ := listFiles(s.root)
		latest := ""
		if len(files) > 0 {
			latest = files[0].Name
		}
		return health.Snapshot{
			Status:                "healthy",
			StorageAvailableBytes: availableStorage(s.root),
			ItemCount:             len(files),
			LatestItem:            latest,
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
	}
	return httpServer.ListenAndServe()
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/files", s.files)
	mux.HandleFunc("/api/upload", s.upload)
	mux.HandleFunc("/files/", s.download)
	return securityHeaders(mux)
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
	files, err := listFiles(s.root)
	if err != nil {
		http.Error(response, "Unable to read received files", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if request.Method == http.MethodHead {
		return
	}
	_ = s.template.Execute(response, pageData{
		Files:            files,
		AvailableStorage: formatBytes(int64(availableStorage(s.root))),
		MaximumFileSize:  formatBytes(s.maximumFileSize),
	})
}

func (s *server) files(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", "GET")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	files, err := listFiles(s.root)
	if err != nil {
		writeDropError(response, http.StatusInternalServerError, "Unable to read received files.")
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(map[string]any{
		"files":                 files,
		"storageAvailableBytes": availableStorage(s.root),
	})
}

func (s *server) download(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name, err := url.PathUnescape(strings.TrimPrefix(request.URL.Path, "/files/"))
	if err != nil {
		http.NotFound(response, request)
		return
	}
	filePath, err := downloadPath(s.root, name)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Disposition", mimeDisposition(name))
	response.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(response, request, filePath)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'self' wails://wails http://127.0.0.1:* http://localhost:*; base-uri 'none'; form-action 'self'")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(response, request)
	})
}

func pathEscape(value string) string {
	return url.PathEscape(value)
}

func integerString(value int) string {
	return strconv.Itoa(value)
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "Unavailable"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	number := float64(value)
	unit := 0
	for number >= 1000 && unit < len(units)-1 {
		number /= 1000
		unit++
	}
	if number >= 10 || unit == 0 {
		return fmt.Sprintf("%.0f %s", number, units[unit])
	}
	return fmt.Sprintf("%.1f %s", number, units[unit])
}

func mimeDisposition(name string) string {
	return `attachment; filename="` + strings.ReplaceAll(name, `"`, "") + `"; filename*=UTF-8''` + url.QueryEscape(name)
}

const dropPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Drop · Spare</title>
  <style>
    :root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, sans-serif; color: #17221c; background: #f5f7f4; }
    * { box-sizing: border-box; }
    body { margin: 0; min-width: 0; }
    a { color: #185d3a; text-underline-offset: .18em; overflow-wrap: anywhere; }
    button, input { font: inherit; }
    button { min-height: 44px; border: 0; border-radius: 12px; padding-inline: 18px; background: #185d3a; color: white; font-weight: 650; cursor: pointer; }
    button:disabled { opacity: .55; cursor: wait; }
    :focus-visible { outline: 3px solid #0b67c2; outline-offset: 3px; }
    .skip { position: fixed; inset-block-start: 8px; inset-inline-start: 8px; transform: translateY(-160%); background: white; padding: 10px 14px; border-radius: 10px; z-index: 2; }
    .skip:focus { transform: translateY(0); }
    header, main { width: min(100% - 32px, 760px); margin-inline: auto; }
    header { padding-block: 28px 18px; font-weight: 750; }
    main { padding-block-end: 48px; }
    .hero { padding-block: 40px 48px; }
    .eyebrow { margin: 0 0 12px; color: #32684a; font-weight: 700; }
    h1 { margin: 0; font-size: clamp(2.35rem, 10vw, 4.8rem); line-height: .98; letter-spacing: -.055em; text-wrap: balance; }
    .lede { max-width: 590px; margin: 20px 0 0; font-size: 1.12rem; line-height: 1.6; color: #536259; }
    .grid { display: grid; gap: 24px; }
    .card { background: white; border-radius: 22px; padding: clamp(20px, 5vw, 32px); box-shadow: 0 0 0 1px rgb(0 0 0 / 6%), 0 2px 8px rgb(0 0 0 / 5%); }
    h2 { margin: 0; font-size: 1.35rem; }
    .hint { margin: 8px 0 22px; color: #607067; line-height: 1.55; }
    .field { display: grid; gap: 10px; }
    input[type=file] { min-height: 44px; width: 100%; padding: 10px; border: 1px solid #aab5ae; border-radius: 12px; background: white; }
    .actions { display: flex; flex-wrap: wrap; gap: 12px; margin-block-start: 16px; align-items: center; }
    progress { width: min(100%, 360px); height: 16px; accent-color: #185d3a; }
    .status { min-height: 1.5em; margin: 12px 0 0; }
    .summary { display: flex; flex-wrap: wrap; gap: 8px 20px; margin-block-end: 24px; color: #607067; }
    ul { list-style: none; padding: 0; margin: 0; display: grid; gap: 12px; }
    li { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px; min-height: 44px; }
    .file-meta { color: #6b776f; font-size: .9rem; white-space: nowrap; }
    .empty { color: #607067; margin: 0; }
    @media (max-width: 30rem) { li { grid-template-columns: 1fr; gap: 4px; } .file-meta { white-space: normal; } }
    @media (forced-colors: active) { button { border: 1px solid ButtonText; } }
  </style>
</head>
<body>
  <a class="skip" href="#main">Skip to content</a>
  <header>Spare · Drop</header>
  <main id="main">
    <section class="hero">
      <p class="eyebrow">Ready for nearby devices</p>
      <h1>Drop is ready</h1>
      <p class="lede">Send one file at a time to this computer. Files stay in the folder its owner selected.</p>
    </section>
    <div class="grid">
      <section class="card" aria-labelledby="send-heading">
        <h2 id="send-heading">Send a file</h2>
        <p class="hint">Maximum file size: {{.MaximumFileSize}}</p>
        <form id="upload-form" action="/api/upload" method="post" enctype="multipart/form-data">
          <div class="field">
            <label for="file">Choose a file</label>
            <input id="file" name="file" type="file" required>
          </div>
          <div class="actions">
            <button id="send" type="submit">Send file</button>
            <progress id="progress" value="0" max="100" hidden aria-label="Upload progress"></progress>
          </div>
          <p id="status" class="status" role="status" aria-live="polite"></p>
        </form>
      </section>
      <section class="card" aria-labelledby="files-heading">
        <h2 id="files-heading">Received files</h2>
        <div class="summary">
          <span>{{len .Files}} file{{if ne (len .Files) 1}}s{{end}}</span>
          <span>{{.AvailableStorage}} available</span>
        </div>
        {{if .Files}}
        <ul>
          {{range .Files}}
          <li>
            <a href="{{.URL}}">{{.Name}}</a>
            <span class="file-meta">{{formatBytes .Size}}</span>
          </li>
          {{end}}
        </ul>
        {{else}}
        <p class="empty">No files received yet. Choose a file above to send the first one.</p>
        {{end}}
      </section>
    </div>
  </main>
  <script>
    const form = document.querySelector("#upload-form");
    const input = document.querySelector("#file");
    const button = document.querySelector("#send");
    const progress = document.querySelector("#progress");
    const status = document.querySelector("#status");
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      if (!input.files.length) {
        status.textContent = "Choose a file to send.";
        input.focus();
        return;
      }
      const request = new XMLHttpRequest();
      request.open("POST", "/api/upload");
      request.upload.addEventListener("progress", (upload) => {
        if (!upload.lengthComputable) return;
        progress.hidden = false;
        progress.value = Math.round((upload.loaded / upload.total) * 100);
        status.textContent = "Sending " + input.files[0].name + "… " + progress.value + "%";
      });
      request.addEventListener("load", () => {
        button.disabled = false;
        if (request.status >= 200 && request.status < 300) {
          progress.value = 100;
          status.textContent = "File received. Refreshing the file list.";
          window.setTimeout(() => window.location.reload(), 500);
          return;
        }
        let message = "Unable to send this file. Check the connection and try again.";
        try { message = JSON.parse(request.responseText).error || message; } catch (_) {}
        status.textContent = message;
      });
      request.addEventListener("error", () => {
        button.disabled = false;
        status.textContent = "Unable to send this file. Check the connection and try again.";
      });
      button.disabled = true;
      progress.hidden = false;
      progress.value = 0;
      status.textContent = "Starting upload.";
      request.send(new FormData(form));
    });
  </script>
</body>
</html>`
