package recipeview

const viewerPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="icon" href="data:,">
  <title>Recipe viewer · Spare</title>
  <style>
    :root {
      color: oklch(0.23 0.018 150);
      background: oklch(0.965 0.008 92);
      font-family: Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      font-synthesis: none;
      -webkit-font-smoothing: antialiased;
      -moz-osx-font-smoothing: grayscale;
      --ink: oklch(0.23 0.018 150);
      --muted: oklch(0.47 0.018 150);
      --surface: oklch(0.995 0.003 92);
      --surface-muted: oklch(0.94 0.012 100);
      --primary: oklch(0.35 0.075 155);
      --primary-hover: oklch(0.3 0.075 155);
      --success: oklch(0.42 0.1 150);
      --danger: oklch(0.45 0.16 28);
      --focus: oklch(0.52 0.16 255);
      --shadow: 0 0 0 1px oklch(0 0 0 / .06), 0 1px 2px -1px oklch(0 0 0 / .06), 0 12px 36px -20px oklch(0 0 0 / .18);
    }
    * { box-sizing: border-box; }
    html, body { min-width: 20rem; }
    body {
      min-height: 100vh;
      margin: 0;
      background:
        radial-gradient(circle at 80% 0%, oklch(0.92 0.035 145 / .55), transparent 32rem),
        oklch(0.965 0.008 92);
    }
    button, input { font: inherit; }
    button { touch-action: manipulation; }
    :focus-visible { outline: 3px solid var(--focus); outline-offset: 3px; }
    ::selection { background: oklch(0.78 0.08 150); color: var(--ink); }
    .skip {
      position: fixed;
      inset-block-start: 1rem;
      inset-inline-start: 1rem;
      z-index: 10;
      min-height: 2.75rem;
      transform: translateY(-180%);
      border-radius: .75rem;
      padding: .75rem 1rem;
      background: var(--ink);
      color: white;
    }
    .skip:focus { transform: translateY(0); }
    header, main { width: min(100% - 2rem, 76rem); margin-inline: auto; }
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding-block: 1.5rem;
    }
    .brand { display: flex; align-items: center; gap: .7rem; font-weight: 700; letter-spacing: -.01em; }
    .brand-mark {
      display: grid;
      width: 2rem;
      height: 2rem;
      place-items: center;
      border-radius: .65rem;
      background: var(--ink);
      color: white;
      font-size: .82rem;
    }
    .file-name { margin: 0; color: var(--muted); font-size: .88rem; overflow-wrap: anywhere; text-align: end; }
    main { padding-block: clamp(2.5rem, 7vw, 5.5rem); padding-block-end: calc(5rem + env(safe-area-inset-bottom)); }
    .loading, .error {
      display: grid;
      min-height: 25rem;
      place-content: center;
      text-align: center;
    }
    .loading p, .error p { max-width: 38rem; margin: .75rem auto 0; color: var(--muted); line-height: 1.55; }
    .error h1 { color: var(--danger); }
    .eyebrow {
      margin: 0 0 1rem;
      color: var(--muted);
      font-size: .78rem;
      font-weight: 700;
      letter-spacing: .08em;
      text-transform: uppercase;
    }
    h1, h2, h3, p { overflow-wrap: anywhere; }
    h1 {
      max-width: 18ch;
      margin: 0;
      font-size: clamp(2.5rem, 7vw, 5.5rem);
      font-weight: 650;
      letter-spacing: -.06em;
      line-height: .98;
      text-wrap: balance;
    }
    .lede {
      max-width: 43rem;
      margin: 1.4rem 0 0;
      color: var(--muted);
      font-size: clamp(1.05rem, 2vw, 1.3rem);
      line-height: 1.55;
      text-wrap: pretty;
    }
    .badge {
      display: inline-flex;
      min-height: 1.9rem;
      align-items: center;
      gap: .5rem;
      margin-block-start: 1.5rem;
      border-radius: 999px;
      padding-inline: .75rem;
      background: oklch(0.9 0.025 145);
      color: var(--success);
      font-size: .8rem;
      font-weight: 700;
    }
    .badge::before { width: .52rem; height: .52rem; border-radius: 50%; background: currentColor; content: ""; }
    .badge.unsupported { background: oklch(0.93 0.025 28); color: var(--danger); }
    .metrics {
      display: grid;
      max-width: 52rem;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 1px;
      margin: 2.5rem 0 0;
      overflow: hidden;
      border-radius: 1.2rem;
      padding: 0;
      background: oklch(0.88 0.01 145);
      box-shadow: var(--shadow);
    }
    .metrics div { min-width: 0; padding: 1.2rem 1.4rem; background: var(--surface); }
    .metrics dt { color: var(--muted); font-size: .78rem; }
    .metrics dd { margin: .35rem 0 0; font-size: 1.35rem; font-weight: 700; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
    .workspace {
      display: grid;
      grid-template-columns: minmax(17rem, .72fr) minmax(0, 1.55fr);
      align-items: start;
      gap: 1rem;
      margin-block-start: clamp(3.5rem, 8vw, 6rem);
    }
    .panel { min-width: 0; border-radius: 1.35rem; background: var(--surface); box-shadow: var(--shadow); }
    .sidebar { overflow: hidden; }
    .sidebar-heading { padding: 1.35rem 1.35rem .8rem; }
    .sidebar-heading h2 { margin: 0; font-size: 1.3rem; letter-spacing: -.025em; }
    .sidebar-heading p { margin: .55rem 0 0; color: var(--muted); font-size: .86rem; line-height: 1.5; }
    .search-field { display: grid; gap: .45rem; padding: .65rem 1rem .85rem; }
    .search-field label { font-size: .82rem; font-weight: 650; }
    .search-field input {
      width: 100%;
      min-height: 2.75rem;
      border: 1px solid oklch(0.72 0.018 150);
      border-radius: .8rem;
      padding-inline: .85rem;
      background: white;
      color: var(--ink);
      font-size: 1rem;
    }
    .file-list { display: grid; grid-template-columns: minmax(0, 1fr); gap: .35rem; margin: 0; padding: 0 .7rem .7rem; list-style: none; }
    .file-list li { min-width: 0; }
    .file-button {
      display: grid;
      width: 100%;
      min-height: 3.7rem;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
      gap: .75rem;
      border: 0;
      border-radius: .85rem;
      padding: .75rem .85rem;
      background: transparent;
      color: var(--ink);
      text-align: start;
      cursor: pointer;
    }
    .file-button:hover { background: var(--surface-muted); }
    .file-button[aria-current="true"] { background: oklch(0.9 0.025 145); box-shadow: inset 3px 0 var(--primary); }
    .file-button code { min-width: 0; font-size: .8rem; overflow-wrap: anywhere; }
    .file-size { color: var(--muted); font-size: .72rem; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .empty-list { margin: .3rem; border-radius: .8rem; padding: 1rem; background: var(--surface-muted); color: var(--muted); line-height: 1.5; }
    .detail { min-height: 28rem; padding: clamp(1.25rem, 4vw, 2rem); }
    .detail h2 { margin: 0; font-size: clamp(1.55rem, 4vw, 2.3rem); letter-spacing: -.04em; line-height: 1.08; text-wrap: balance; }
    .detail h3 { margin: 0; font-size: 1.05rem; }
    .detail-meta { display: flex; flex-wrap: wrap; gap: .4rem 1rem; margin: .8rem 0 0; color: var(--muted); font-size: .8rem; }
    .detail-section { margin-block-start: 2.5rem; }
    .detail-section > p { max-width: 65ch; color: var(--muted); line-height: 1.55; }
    .summary-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; margin-block-start: 2rem; }
    .summary-card { min-width: 0; border-radius: 1rem; padding: 1.15rem; background: var(--surface-muted); }
    .summary-card h3 { margin-block-end: .9rem; }
    .properties { display: grid; gap: .8rem; margin: 0; }
    .properties div { display: grid; grid-template-columns: minmax(6.5rem, .5fr) minmax(0, 1fr); gap: .75rem; }
    .properties dt { color: var(--muted); font-size: .78rem; }
    .properties dd { min-width: 0; margin: 0; font-size: .86rem; overflow-wrap: anywhere; }
    .permission-list, .config-list { display: grid; gap: .75rem; margin: 0; padding: 0; list-style: none; }
    .permission-list li { display: grid; grid-template-columns: 1.1rem minmax(0, 1fr); gap: .55rem; color: var(--muted); font-size: .86rem; line-height: 1.45; }
    .permission-list li.granted { color: var(--ink); }
    .config-list li { border-radius: .75rem; padding: .85rem; background: var(--surface-muted); }
    .config-list strong { display: block; }
    .config-list span { display: block; margin-block-start: .25rem; color: var(--muted); font-size: .82rem; line-height: 1.45; }
    .checksum {
      display: block;
      margin-block-start: .7rem;
      border-radius: .8rem;
      padding: .9rem;
      background: var(--ink);
      color: oklch(0.96 0.01 145);
      font-size: .75rem;
      overflow-wrap: anywhere;
    }
    pre {
      max-height: 36rem;
      margin: 1rem 0 0;
      overflow: auto;
      border-radius: .9rem;
      padding: 1rem;
      background: var(--ink);
      color: oklch(0.96 0.01 145);
      font-family: "Geist Mono", "IBM Plex Mono", monospace;
      font-size: .82rem;
      line-height: 1.55;
      tab-size: 2;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }
    .image-frame {
      display: grid;
      min-height: 16rem;
      margin-block-start: 1rem;
      place-items: center;
      border-radius: 1rem;
      padding: 1rem;
      background:
        linear-gradient(45deg, oklch(0.9 0 0) 25%, transparent 25%),
        linear-gradient(-45deg, oklch(0.9 0 0) 25%, transparent 25%),
        linear-gradient(45deg, transparent 75%, oklch(0.9 0 0) 75%),
        linear-gradient(-45deg, transparent 75%, oklch(0.9 0 0) 75%),
        white;
      background-position: 0 0, 0 .6rem, .6rem -.6rem, -.6rem 0;
      background-size: 1.2rem 1.2rem;
    }
    .image-frame img { display: block; max-width: 100%; max-height: 32rem; outline: 1px solid oklch(0 0 0 / .1); outline-offset: -1px; }
    .no-preview { margin-block-start: 1rem; border-radius: 1rem; padding: 1.3rem; background: var(--surface-muted); }
    .no-preview strong { display: block; }
    .no-preview p { margin: .45rem 0 0; color: var(--muted); line-height: 1.5; }
    code { font-family: "Geist Mono", "IBM Plex Mono", monospace; font-variant-ligatures: none; }
    [hidden] { display: none !important; }
    @media (prefers-reduced-motion: no-preference) {
      .file-button { transition-property: background-color, scale; transition-duration: 130ms; transition-timing-function: ease-out; }
      .file-button:active { scale: .96; }
    }
    @media (max-width: 50rem) {
      .workspace { grid-template-columns: 1fr; }
      .file-list { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    }
    @media (max-width: 36rem) {
      header { align-items: flex-start; }
      .metrics, .summary-grid { grid-template-columns: 1fr; }
      .file-list { grid-template-columns: minmax(0, 1fr); }
      .properties div { grid-template-columns: 1fr; gap: .25rem; }
    }
    @media (forced-colors: active) {
      .panel, .metrics, .summary-card, .file-button[aria-current="true"], .no-preview { border: 1px solid CanvasText; }
      .file-button { border: 1px solid transparent; }
    }
  </style>
</head>
<body>
  <a class="skip" href="#main">Skip to content</a>
  <header>
    <div class="brand" translate="no"><span class="brand-mark" aria-hidden="true">S</span>Spare · Recipe viewer</div>
    <p id="package-file-name" class="file-name"></p>
  </header>
  <main id="main">
    <section id="loading" class="loading" aria-label="Opening recipe package">
      <div>
        <p class="eyebrow">Reading package</p>
        <h1>Opening recipe</h1>
        <p>Checking the manifest and package contents before showing them.</p>
      </div>
    </section>
    <section id="error" class="error" hidden role="alert">
      <div>
        <p class="eyebrow">Package viewer</p>
        <h1>Unable to open recipe</h1>
        <p id="error-message">Close this tab and run the viewer again.</p>
      </div>
    </section>
    <div id="app" hidden>
      <section aria-labelledby="recipe-name">
        <p class="eyebrow">Validated .sp package</p>
        <h1 id="recipe-name"></h1>
        <p id="recipe-description" class="lede"></p>
        <span id="compatibility" class="badge"></span>
        <dl class="metrics" aria-label="Package summary">
          <div><dt>Package size</dt><dd id="package-size"></dd></div>
          <div><dt>Unpacked size</dt><dd id="unpacked-size"></dd></div>
          <div><dt>Files</dt><dd id="file-count"></dd></div>
        </dl>
      </section>

      <div class="workspace">
        <aside class="panel sidebar" aria-labelledby="contents-heading">
          <div class="sidebar-heading">
            <h2 id="contents-heading">Package contents</h2>
            <p>Select a file to preview it safely.</p>
          </div>
          <div class="search-field">
            <label for="file-search">Filter files</label>
            <input id="file-search" type="search" placeholder="README, icon, binary…" autocomplete="off">
          </div>
          <ol id="file-list" class="file-list"></ol>
        </aside>
        <section id="detail" class="panel detail"></section>
      </div>
    </div>
  </main>
  <script>
    const loading = document.querySelector("#loading");
    const errorPanel = document.querySelector("#error");
    const errorMessage = document.querySelector("#error-message");
    const app = document.querySelector("#app");
    const fileList = document.querySelector("#file-list");
    const search = document.querySelector("#file-search");
    const detail = document.querySelector("#detail");
    let packageData;
    let selected = "overview";

    function element(tag, className, text) {
      const node = document.createElement(tag);
      if (className) node.className = className;
      if (text !== undefined) node.textContent = text;
      return node;
    }

    function formatBytes(value) {
      if (!value) return "0 B";
      const units = ["B", "KB", "MB", "GB"];
      let amount = value;
      let unit = 0;
      while (amount >= 1000 && unit < units.length - 1) {
        amount /= 1000;
        unit += 1;
      }
      return amount.toFixed(amount >= 10 || unit === 0 ? 0 : 1) + " " + units[unit];
    }

    function property(list, label, value) {
      const row = element("div");
      row.append(element("dt", "", label), element("dd", "", value || "Not declared"));
      list.append(row);
    }

    async function request(path) {
      const response = await fetch(path);
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error?.message || "Unable to read this package.");
      }
      return response;
    }

    async function load() {
      try {
        const response = await request("/api/package");
        packageData = await response.json();
        renderShell();
        renderFiles();
        renderOverview();
        loading.hidden = true;
        app.hidden = false;
      } catch (loadError) {
        loading.hidden = true;
        errorPanel.hidden = false;
        errorMessage.textContent = loadError instanceof Error ? loadError.message : "Close this tab and run the viewer again.";
      }
    }

    function renderShell() {
      const manifest = packageData.manifest;
      document.title = manifest.name + " · Spare recipe";
      document.querySelector("#package-file-name").textContent = packageData.fileName;
      document.querySelector("#recipe-name").textContent = manifest.name + " " + manifest.version;
      document.querySelector("#recipe-description").textContent = manifest.description;
      document.querySelector("#package-size").textContent = formatBytes(packageData.packageSize);
      document.querySelector("#unpacked-size").textContent = formatBytes(packageData.uncompressedSize);
      document.querySelector("#file-count").textContent = String(packageData.files.length);
      const compatibility = document.querySelector("#compatibility");
      compatibility.textContent = packageData.compatibility.supported
        ? packageData.compatibility.rating + " on this computer"
        : "Unsupported on this computer";
      compatibility.classList.toggle("unsupported", !packageData.compatibility.supported);
    }

    function renderFiles() {
      fileList.replaceChildren();
      const query = search.value.trim().toLocaleLowerCase();
      const visible = packageData.files.filter((file) => file.name.toLocaleLowerCase().includes(query));
      if (!query) {
        const item = element("li");
        const button = element("button", "file-button");
        button.type = "button";
        button.setAttribute("aria-current", selected === "overview" ? "true" : "false");
        button.append(element("strong", "", "Recipe overview"), element("span", "file-size", "Manifest"));
        button.addEventListener("click", () => {
          selected = "overview";
          renderFiles();
          renderOverview();
        });
        item.append(button);
        fileList.append(item);
      }
      for (const file of visible) {
        const item = element("li");
        const button = element("button", "file-button");
        button.type = "button";
        button.setAttribute("aria-current", selected === file.name ? "true" : "false");
        button.setAttribute("aria-label", file.name + ", " + formatBytes(file.size));
        button.append(element("code", "", file.name), element("span", "file-size", formatBytes(file.size)));
        button.addEventListener("click", () => {
          selected = file.name;
          renderFiles();
          void renderFile(file);
        });
        item.append(button);
        fileList.append(item);
      }
      if (!visible.length) {
        fileList.append(element("li", "empty-list", "No package files match this filter."));
      }
    }

    function renderOverview() {
      const manifest = packageData.manifest;
      detail.replaceChildren();
      detail.append(element("p", "eyebrow", "Recipe overview"), element("h2", "", manifest.name));
      const meta = element("p", "detail-meta");
      meta.append(
        element("span", "", "ID " + manifest.id),
        element("span", "", "Version " + manifest.version),
        element("span", "", "Schema " + manifest.schema)
      );
      detail.append(meta);

      const grid = element("div", "summary-grid");
      const runtimeCard = element("section", "summary-card");
      runtimeCard.append(element("h3", "", "Runtime and support"));
      const runtime = element("dl", "properties");
      property(runtime, "Runtime", manifest.runtime.type);
      property(runtime, "Systems", manifest.support.systems.join(", "));
      property(runtime, "Architectures", manifest.support.architectures.join(", "));
      property(runtime, "Network", manifest.network.visibility || "None");
      runtimeCard.append(runtime);

      const storageCard = element("section", "summary-card");
      storageCard.append(element("h3", "", "Storage and health"));
      const storage = element("dl", "properties");
      property(storage, "Folder field", manifest.storage.pathField || "Not used");
      property(storage, "Folder access", manifest.storage.pathField ? (manifest.storage.readOnly ? "Read-only" : "Read and write") : "Not used");
      property(storage, "Health check", manifest.health.type + " " + manifest.health.path);
      property(storage, "Memory", formatBytes(manifest.resources.memoryRecommendedBytes) + " recommended");
      storageCard.append(storage);
      grid.append(runtimeCard, storageCard);
      detail.append(grid);

      const permissionsSection = element("section", "detail-section");
      permissionsSection.append(element("h3", "", "Permissions"));
      const permissions = element("ul", "permission-list");
      for (const permission of packageData.permissions) {
        const item = element("li", permission.granted ? "granted" : "");
        item.append(element("span", "", permission.granted ? "✓" : "—"), element("span", "", permission.description + (permission.granted ? "" : " — not allowed")));
        permissions.append(item);
      }
      permissionsSection.append(permissions);
      detail.append(permissionsSection);

      const configSection = element("section", "detail-section");
      configSection.append(element("h3", "", "Configuration"));
      const config = element("ul", "config-list");
      const fields = Object.entries(manifest.config || {});
      if (!fields.length) {
        config.append(element("li", "", "This recipe has no configuration fields."));
      } else {
        for (const [id, field] of fields.sort(([left], [right]) => left.localeCompare(right))) {
          const item = element("li");
          item.append(element("strong", "", field.label), element("span", "", id + " · " + field.type + (field.required ? " · required" : " · optional")));
          if (field.description) item.append(element("span", "", field.description));
          config.append(item);
        }
      }
      configSection.append(config);
      detail.append(configSection);

      const integrity = element("section", "detail-section");
      integrity.append(element("h3", "", "Package checksum"), element("code", "checksum", packageData.sha256));
      detail.append(integrity);
    }

    async function renderFile(file) {
      detail.replaceChildren();
      detail.append(element("p", "eyebrow", "Package file"), element("h2", "", file.name));
      const meta = element("p", "detail-meta");
      meta.append(
        element("span", "", formatBytes(file.size)),
        element("span", "", formatBytes(file.compressedSize) + " compressed"),
        element("span", "", file.preview === "none" ? "No safe preview" : file.preview === "image" ? "Image preview" : "Text preview")
      );
      detail.append(meta);
      if (file.preview === "none") {
        const empty = element("div", "no-preview");
        empty.append(
          element("strong", "", "Preview unavailable"),
          element("p", "", file.size > 2097152
            ? "This file is larger than the 2 MB preview limit. It remains listed in the package."
            : "This file type is listed but not opened in the browser, which keeps packaged executables and unknown data inert.")
        );
        detail.append(empty);
        return;
      }
      try {
        if (file.preview === "image") {
          const frame = element("div", "image-frame");
          const image = element("img");
          image.src = "/api/file?name=" + encodeURIComponent(file.name);
          image.alt = "Preview of " + file.name;
          frame.append(image);
          detail.append(frame);
          return;
        }
        const response = await request("/api/file?name=" + encodeURIComponent(file.name));
        detail.append(element("pre", "", await response.text()));
      } catch (previewError) {
        const empty = element("div", "no-preview");
        empty.append(
          element("strong", "", "Unable to preview this file"),
          element("p", "", previewError instanceof Error ? previewError.message : "Select another file.")
        );
        detail.append(empty);
      }
    }

    search.addEventListener("input", renderFiles);
    void load();
    window.setInterval(() => {
      void fetch("/api/heartbeat", { method: "POST", keepalive: true });
    }, 15000);
  </script>
</body>
</html>`
