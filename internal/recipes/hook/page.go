package hook

const hookPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="icon" href="data:,">
  <title>Hook · Spare</title>
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
    .status { display: inline-flex; align-items: center; gap: .55rem; margin: 0; color: var(--muted); font-size: .88rem; }
    .status::before {
      width: .62rem;
      height: .62rem;
      border-radius: 50%;
      background: var(--success);
      box-shadow: 0 0 0 .24rem oklch(0.42 0.1 150 / .11);
      content: "";
    }
    main { padding-block: clamp(2.5rem, 8vw, 6rem); padding-block-end: calc(5rem + env(safe-area-inset-bottom)); }
    .hero { max-width: 53rem; }
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
      max-width: 16ch;
      margin: 0;
      font-size: clamp(2.5rem, 7vw, 5.75rem);
      font-weight: 650;
      letter-spacing: -.065em;
      line-height: .98;
      text-wrap: balance;
    }
    .lede {
      max-width: 43rem;
      margin: 1.5rem 0 0;
      color: var(--muted);
      font-size: clamp(1.05rem, 2vw, 1.3rem);
      line-height: 1.55;
      text-wrap: pretty;
    }
    .endpoint-card {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
      gap: 1rem;
      max-width: 53rem;
      margin-block-start: 2rem;
      border-radius: 1.35rem;
      padding: 1rem 1rem 1rem 1.25rem;
      background: var(--surface);
      box-shadow: var(--shadow);
    }
    .endpoint-label { display: block; margin-block-end: .4rem; color: var(--muted); font-size: .78rem; font-weight: 650; }
    code, pre, .request-path {
      font-family: "Geist Mono", "IBM Plex Mono", monospace;
      font-variant-ligatures: none;
    }
    #endpoint { display: block; overflow-wrap: anywhere; }
    .button {
      display: inline-flex;
      min-height: 2.75rem;
      align-items: center;
      justify-content: center;
      border: 0;
      border-radius: .8rem;
      padding-inline: 1.1rem;
      font-weight: 650;
      cursor: pointer;
    }
    .button-primary { background: var(--primary); color: white; box-shadow: 0 1px 2px oklch(0 0 0 / .16); }
    .button-secondary { background: var(--surface-muted); color: var(--ink); }
    .button:disabled { cursor: wait; opacity: .58; }
    .workspace {
      display: grid;
      grid-template-columns: minmax(17rem, .7fr) minmax(0, 1.5fr);
      align-items: start;
      gap: 1rem;
      margin-block-start: clamp(3.5rem, 8vw, 6rem);
    }
    .panel {
      min-width: 0;
      border-radius: 1.35rem;
      background: var(--surface);
      box-shadow: var(--shadow);
    }
    .panel-heading { padding: 1.4rem 1.4rem 1rem; }
    .panel-heading-row { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; }
    .panel-heading h2 { margin: 0; font-size: 1.35rem; letter-spacing: -.025em; }
    .count { color: var(--muted); font-size: .82rem; font-variant-numeric: tabular-nums; }
    .panel-heading p { margin: .55rem 0 0; color: var(--muted); font-size: .9rem; line-height: 1.5; }
    .request-list { display: grid; grid-template-columns: minmax(0, 1fr); gap: .55rem; margin: 0; padding: .35rem .7rem .7rem; list-style: none; }
    .request-item { min-width: 0; }
    .request-item button {
      width: 100%;
      min-height: 4.75rem;
      border: 0;
      border-radius: .9rem;
      padding: .9rem;
      background: transparent;
      color: var(--ink);
      text-align: start;
      cursor: pointer;
    }
    .request-item button:hover { background: var(--surface-muted); }
    .request-item button[aria-current="true"] { background: oklch(0.9 0.025 145); box-shadow: inset 3px 0 var(--primary); }
    .request-line { display: flex; min-width: 0; align-items: center; gap: .65rem; }
    .method {
      display: inline-flex;
      flex: 0 0 auto;
      min-width: 3.25rem;
      min-height: 1.6rem;
      align-items: center;
      justify-content: center;
      border-radius: .48rem;
      padding-inline: .45rem;
      background: var(--ink);
      color: white;
      font-size: .68rem;
      font-weight: 750;
      letter-spacing: .035em;
    }
    .request-path { min-width: 0; overflow: hidden; font-size: .86rem; text-overflow: ellipsis; white-space: nowrap; }
    .request-meta { display: flex; flex-wrap: wrap; gap: .35rem .8rem; margin-block-start: .65rem; color: var(--muted); font-size: .76rem; }
    .request-meta time, .duration { font-variant-numeric: tabular-nums; }
    .empty {
      margin: .35rem .7rem .7rem;
      border-radius: .9rem;
      padding: 1.4rem;
      background: var(--surface-muted);
      color: var(--muted);
      line-height: 1.55;
    }
    .empty strong { display: block; margin-block-end: .4rem; color: var(--ink); }
    .detail { min-height: 24rem; padding: clamp(1.25rem, 4vw, 2rem); }
    .detail-empty { display: grid; min-height: 20rem; place-content: center; text-align: center; }
    .detail-empty p { max-width: 34rem; margin: .5rem auto 0; color: var(--muted); line-height: 1.55; }
    .detail-top { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
    .detail h2 { margin: 0; font-size: clamp(1.45rem, 4vw, 2.15rem); letter-spacing: -.035em; line-height: 1.1; }
    .detail h3 { margin: 0; font-size: 1.05rem; }
    .detail-path { margin-block-start: .6rem; color: var(--muted); font-size: .92rem; overflow-wrap: anywhere; }
    .detail-meta { display: flex; flex-wrap: wrap; gap: .45rem 1.2rem; margin: 1.35rem 0 0; color: var(--muted); font-size: .82rem; }
    .detail-section { margin-block-start: 2.5rem; }
    .detail-section-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; margin-block-end: .85rem; }
    .detail-section-heading span { color: var(--muted); font-size: .78rem; }
    pre {
      max-height: 24rem;
      margin: 0;
      overflow: auto;
      border-radius: .9rem;
      padding: 1rem;
      background: var(--ink);
      color: oklch(0.96 0.01 145);
      font-size: .82rem;
      line-height: 1.55;
      tab-size: 2;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }
    .empty-body { min-height: 4rem; color: oklch(0.8 0.01 145); }
    .headers { display: grid; gap: .8rem; margin: 0; }
    .header-row { display: grid; grid-template-columns: minmax(7rem, .35fr) minmax(0, 1fr); gap: 1rem; }
    .headers dt { color: var(--muted); font-size: .8rem; overflow-wrap: anywhere; }
    .headers dd { min-width: 0; margin: 0; font-family: "Geist Mono", "IBM Plex Mono", monospace; font-size: .82rem; overflow-wrap: anywhere; }
    .replay-form { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: .75rem; }
    .field { display: grid; gap: .5rem; }
    label { font-size: .86rem; font-weight: 650; }
    input {
      width: 100%;
      min-height: 2.75rem;
      border: 1px solid oklch(0.72 0.018 150);
      border-radius: .8rem;
      padding-inline: .85rem;
      background: white;
      color: var(--ink);
      font-size: 1rem;
    }
    input[aria-invalid="true"] { border-color: var(--danger); }
    .field-hint { margin: 0; color: var(--muted); font-size: .78rem; line-height: 1.45; }
    .form-error { min-height: 1.3rem; margin: .65rem 0 0; color: var(--danger); font-size: .82rem; }
    .replay-list { display: grid; gap: .75rem; margin: 1rem 0 0; padding: 0; list-style: none; }
    .replay-card { border-radius: .9rem; padding: 1rem; background: var(--surface-muted); }
    .replay-summary { display: flex; flex-wrap: wrap; align-items: center; gap: .45rem .75rem; }
    .replay-state { font-weight: 700; }
    .replay-state.completed { color: var(--success); }
    .replay-state.failed { color: var(--danger); }
    .replay-target { margin: .55rem 0 0; font-family: "Geist Mono", "IBM Plex Mono", monospace; font-size: .8rem; overflow-wrap: anywhere; }
    .replay-error { margin: .55rem 0 0; color: var(--danger); font-size: .85rem; }
    .replay-response { margin-block-start: .75rem; }
    .sr-only {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      clip-path: inset(50%);
      white-space: nowrap;
    }
    @media (prefers-reduced-motion: no-preference) {
      .button { transition-property: background-color, scale; transition-duration: 130ms; transition-timing-function: ease-out; }
      .button:not(:disabled):active { scale: .96; }
      .button-primary:hover { background: var(--primary-hover); }
    }
    @media (max-width: 50rem) {
      .workspace { grid-template-columns: 1fr; }
      .request-list { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    }
    @media (max-width: 35rem) {
      header { align-items: flex-start; }
      .status { margin-block-start: .5rem; }
      .endpoint-card, .replay-form { grid-template-columns: 1fr; align-items: stretch; }
      .request-list { grid-template-columns: minmax(0, 1fr); }
      .detail-top { display: block; }
      .header-row { grid-template-columns: 1fr; gap: .25rem; }
    }
    @media (forced-colors: active) {
      .panel, .endpoint-card, .request-item button[aria-current="true"] { border: 1px solid CanvasText; }
      .method, .button { border: 1px solid ButtonText; }
    }
  </style>
</head>
<body>
  <a class="skip" href="#main">Skip to content</a>
  <header>
    <div class="brand" translate="no"><span class="brand-mark" aria-hidden="true">S</span>Spare · Hook</div>
    <p class="status">Listening</p>
  </header>
  <main id="main">
    <section class="hero" aria-labelledby="page-title">
      <p class="eyebrow">Local webhook inbox</p>
      <h1 id="page-title">See every request</h1>
      <p class="lede">Send a webhook to this endpoint, inspect exactly what arrived, then replay it to another service.</p>
      <div class="endpoint-card">
        <div>
          <span class="endpoint-label">Webhook endpoint</span>
          <code id="endpoint"></code>
        </div>
        <button id="copy-endpoint" class="button button-secondary" type="button">Copy endpoint</button>
      </div>
    </section>

    <div class="workspace">
      <section class="panel" aria-labelledby="requests-heading">
        <div class="panel-heading">
          <div class="panel-heading-row">
            <h2 id="requests-heading">Requests</h2>
            <span id="request-count" class="count">0 received</span>
          </div>
          <p>Newest requests appear first. History keeps the latest 50 while Hook is running.</p>
        </div>
        <ol id="request-list" class="request-list"></ol>
      </section>
      <section id="detail" class="panel detail" aria-live="polite"></section>
    </div>
    <div id="announcer" class="sr-only" role="status"></div>
  </main>

  <script>
    const endpoint = window.location.origin + "/hook";
    const endpointElement = document.querySelector("#endpoint");
    const copyButton = document.querySelector("#copy-endpoint");
    const listElement = document.querySelector("#request-list");
    const countElement = document.querySelector("#request-count");
    const detailElement = document.querySelector("#detail");
    const announcer = document.querySelector("#announcer");
    let requests = [];
    let selectedID = "";
    let renderedID = "";
    let renderedReplayCount = -1;
    let firstLoad = true;

    endpointElement.textContent = endpoint;
    copyButton.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(endpoint);
        announce("Webhook endpoint copied.");
      } catch (_) {
        const area = document.createElement("textarea");
        area.value = endpoint;
        area.setAttribute("readonly", "");
        area.style.position = "fixed";
        area.style.opacity = "0";
        document.body.append(area);
        area.select();
        document.execCommand("copy");
        area.remove();
        announce("Webhook endpoint copied.");
      }
    });

    function element(tag, className, text) {
      const node = document.createElement(tag);
      if (className) node.className = className;
      if (text !== undefined) node.textContent = text;
      return node;
    }

    function announce(message) {
      announcer.textContent = "";
      window.setTimeout(() => { announcer.textContent = message; }, 10);
    }

    function formatTime(value) {
      const date = new Date(value);
      if (Number.isNaN(date.valueOf())) return "Time unavailable";
      return new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "medium"
      }).format(date);
    }

    function formatRelativeTime(value) {
      const seconds = Math.max(0, Math.round((Date.now() - new Date(value).valueOf()) / 1000));
      if (seconds < 10) return "Just now";
      if (seconds < 60) return seconds + " seconds ago";
      const minutes = Math.floor(seconds / 60);
      if (minutes < 60) return minutes + (minutes === 1 ? " minute ago" : " minutes ago");
      return formatTime(value);
    }

    function formatBytes(value) {
      if (!value) return "No body";
      if (value < 1000) return value + (value === 1 ? " byte" : " bytes");
      if (value < 1000000) return (value / 1000).toFixed(value >= 10000 ? 0 : 1) + " KB";
      return (value / 1000000).toFixed(1) + " MB";
    }

    function requestTarget(request) {
      return request.path + (request.query ? "?" + request.query : "");
    }

    async function fetchJSON(path, options) {
      const response = await fetch(path, options);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(data.error?.message || "Unable to load Hook.");
      }
      return data;
    }

    async function loadRequests() {
      try {
        const data = await fetchJSON("/api/requests");
        const previousCount = requests.length;
        requests = data.requests || [];
        if (selectedID && !requests.some((request) => request.id === selectedID)) selectedID = "";
        if (!selectedID && requests.length) selectedID = requests[0].id;
        renderList();
        const selected = requests.find((request) => request.id === selectedID);
        if (selectedID && (renderedID !== selectedID || renderedReplayCount !== selected?.replayCount)) {
          await loadDetail(selectedID);
        } else if (!selectedID) {
          renderEmptyDetail();
        }
        if (!firstLoad && requests.length > previousCount) {
          announce(requests.length - previousCount === 1 ? "New webhook request received." : "New webhook requests received.");
        }
      } catch (error) {
        renderLoadError(error instanceof Error ? error.message : "Unable to load Hook.");
      } finally {
        firstLoad = false;
      }
    }

    function renderList() {
      listElement.replaceChildren();
      countElement.textContent = requests.length + (requests.length === 1 ? " received" : " received");
      if (!requests.length) {
        const empty = element("li", "empty");
        empty.append(
          element("strong", "", "Waiting for the first request"),
          element("span", "", "Send any HTTP method to the webhook endpoint above.")
        );
        listElement.append(empty);
        return;
      }
      for (const request of requests) {
        const item = element("li", "request-item");
        const button = element("button");
        button.type = "button";
        button.setAttribute("aria-current", request.id === selectedID ? "true" : "false");
        button.setAttribute("aria-label", request.method + " " + requestTarget(request) + ", received " + formatTime(request.receivedAt));
        const line = element("span", "request-line");
        line.append(
          element("span", "method", request.method),
          element("span", "request-path", requestTarget(request))
        );
        const meta = element("span", "request-meta");
        const received = element("time", "", formatRelativeTime(request.receivedAt));
        received.dateTime = request.receivedAt;
        meta.append(
          received,
          element("span", "", formatBytes(request.bodySize)),
          element("span", "", request.remoteAddress || "Unknown source")
        );
        if (request.replayCount) meta.append(element("span", "", request.replayCount + (request.replayCount === 1 ? " replay" : " replays")));
        button.append(line, meta);
        button.addEventListener("click", async () => {
          selectedID = request.id;
          renderList();
          await loadDetail(request.id);
        });
        item.append(button);
        listElement.append(item);
      }
    }

    async function loadDetail(id) {
      try {
        const request = await fetchJSON("/api/requests/" + encodeURIComponent(id));
        if (selectedID === id) renderDetail(request);
      } catch (error) {
        renderLoadError(error instanceof Error ? error.message : "Unable to inspect this request.");
      }
    }

    function renderEmptyDetail() {
      renderedID = "";
      renderedReplayCount = -1;
      detailElement.replaceChildren();
      const empty = element("div", "detail-empty");
      empty.append(
        element("h2", "", "No requests yet"),
        element("p", "", "Hook will show the method, path, headers, and body when a webhook arrives.")
      );
      detailElement.append(empty);
    }

    function renderLoadError(message) {
      renderedID = "";
      renderedReplayCount = -1;
      detailElement.replaceChildren();
      const empty = element("div", "detail-empty");
      empty.append(
        element("h2", "", "Unable to load requests"),
        element("p", "", message + " Refresh the page to try again.")
      );
      detailElement.append(empty);
    }

    function prettyBody(request) {
      if (!request.body) return "";
      if (request.bodyEncoding === "base64") return request.body;
      const contentType = (request.headers?.["Content-Type"] || request.headers?.["content-type"] || []).join(" ");
      if (contentType.includes("json")) {
        try { return JSON.stringify(JSON.parse(request.body), null, 2); } catch (_) {}
      }
      return request.body;
    }

    function sectionHeading(title, trailing) {
      const heading = element("div", "detail-section-heading");
      heading.append(element("h3", "", title));
      if (trailing) heading.append(element("span", "", trailing));
      return heading;
    }

    function renderDetail(request) {
      request.replays = request.replays || [];
      renderedID = request.id;
      renderedReplayCount = request.replays.length;
      detailElement.replaceChildren();
      const top = element("div", "detail-top");
      const titleGroup = element("div");
      const eyebrow = element("p", "eyebrow", "Captured request");
      const title = element("h2");
      title.append(element("span", "method", request.method), document.createTextNode(" " + request.path));
      titleGroup.append(eyebrow, title);
      if (request.query) titleGroup.append(element("p", "detail-path request-path", "?" + request.query));
      top.append(titleGroup);

      const meta = element("div", "detail-meta");
      const received = element("time", "", formatTime(request.receivedAt));
      received.dateTime = request.receivedAt;
      meta.append(
        received,
        element("span", "", formatBytes(request.bodySize)),
        element("span", "", "From " + (request.remoteAddress || "unknown source")),
        element("span", "", "Host " + request.host)
      );

      const bodySection = element("section", "detail-section");
      bodySection.append(sectionHeading("Body", request.bodyEncoding === "base64" ? "Base64 encoded" : formatBytes(request.bodySize)));
      const body = element("pre", request.body ? "" : "empty-body", request.body ? prettyBody(request) : "This request has no body.");
      bodySection.append(body);

      const headersSection = element("section", "detail-section");
      headersSection.append(sectionHeading("Headers", Object.keys(request.headers || {}).length + " fields"));
      const headers = element("dl", "headers");
      for (const name of Object.keys(request.headers || {}).sort((a, b) => a.localeCompare(b))) {
        const row = element("div", "header-row");
        row.append(element("dt", "", name), element("dd", "", request.headers[name].join("\n")));
        headers.append(row);
      }
      if (!headers.children.length) headers.append(element("div", "empty", "No request headers were sent."));
      headersSection.append(headers);

      const replaySection = element("section", "detail-section");
      replaySection.append(sectionHeading("Replay", request.replays.length ? request.replays.length + (request.replays.length === 1 ? " attempt" : " attempts") : ""));
      const form = element("form", "replay-form");
      const field = element("div", "field");
      const label = element("label", "", "Destination URL");
      label.htmlFor = "replay-target";
      const input = element("input");
      input.id = "replay-target";
      input.name = "targetUrl";
      input.type = "url";
      input.autocomplete = "url";
      input.placeholder = "https://api.example.com/webhooks";
      input.required = true;
      input.setAttribute("aria-describedby", "replay-hint replay-error");
      field.append(
        label,
        input,
        Object.assign(element("p", "field-hint", "Hook sends the same method, headers, and body to this exact URL."), { id: "replay-hint" })
      );
      const submit = element("button", "button button-primary", "Replay request");
      submit.type = "submit";
      form.append(field, submit);
      const error = element("p", "form-error");
      error.id = "replay-error";
      error.setAttribute("role", "status");
      form.addEventListener("submit", async (event) => {
        event.preventDefault();
        input.setAttribute("aria-invalid", "false");
        error.textContent = "";
        if (!input.checkValidity()) {
          input.setAttribute("aria-invalid", "true");
          error.textContent = "Enter a full URL beginning with http:// or https://.";
          input.focus();
          return;
        }
        submit.disabled = true;
        submit.textContent = "Replaying request…";
        try {
          const data = await fetchJSON("/api/requests/" + encodeURIComponent(request.id) + "/replay", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ targetUrl: input.value })
          });
          const result = data.replay;
          announce(result.status === "completed" ? "Request replayed. Destination returned HTTP " + result.statusCode + "." : result.error);
          await loadRequests();
        } catch (replayError) {
          const message = replayError instanceof Error ? replayError.message : "Unable to replay this request.";
          error.textContent = message;
          announce(message);
        } finally {
          submit.disabled = false;
          submit.textContent = "Replay request";
        }
      });
      replaySection.append(form, error, renderReplays(request.replays));

      detailElement.append(top, meta, bodySection, headersSection, replaySection);
    }

    function renderReplays(replays) {
      const list = element("ol", "replay-list");
      for (const replay of replays) {
        const item = element("li", "replay-card");
        const summary = element("div", "replay-summary");
        const stateLabel = replay.status === "completed" ? "Completed" : "Failed";
        summary.append(
          element("span", "replay-state " + replay.status, stateLabel),
          replay.statusCode ? element("span", "", "HTTP " + replay.statusCode) : document.createTextNode(""),
          element("span", "duration", replay.durationMs + " ms"),
          element("span", "", formatTime(replay.createdAt))
        );
        item.append(summary, element("p", "replay-target", replay.targetUrl));
        if (replay.error) item.append(element("p", "replay-error", replay.error));
        if (replay.responseBody) {
          const details = element("details", "replay-response");
          details.append(
            element("summary", "", replay.bodyTruncated ? "Show truncated response body" : "Show response body"),
            element("pre", "", replay.responseBody)
          );
          item.append(details);
        }
        list.append(item);
      }
      return list;
    }

    void loadRequests();
    window.setInterval(() => void loadRequests(), 1500);
  </script>
</body>
</html>`
