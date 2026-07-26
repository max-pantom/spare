import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import QRCode from "qrcode";
import { loadDashboard, startSite, stopSite } from "./api";
import type { Instance, Machine } from "./types";

type DashboardState = {
  machine?: Machine;
  instance?: Instance;
};

const statusLabels: Record<Instance["status"], string> = {
  starting: "Starting",
  healthy: "Ready",
  degraded: "Needs attention",
  stopped: "Stopped",
  failed: "Needs attention",
  removing: "Removing"
};

export function App() {
  const [data, setData] = useState<DashboardState>({});
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [announcement, setAnnouncement] = useState("");
  const [qrCode, setQrCode] = useState("");
  const previousStatus = useRef<string | undefined>(undefined);

  const refresh = useCallback(async (announce = false) => {
    try {
      const next = await loadDashboard();
      setData(next);
      setError("");
      const status = next.instance?.status ?? "ready";
      if (announce && previousStatus.current && previousStatus.current !== status) {
        setAnnouncement(
          next.instance
            ? `Site status changed to ${statusLabels[next.instance.status]}.`
            : "This computer is ready for a job."
        );
      }
      previousStatus.current = status;
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Unable to load Spare. Check that the local service is running."
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(true), 2_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  const instance = data.instance;
  const shareURL = useMemo(
    () =>
      instance?.urls.find(
        (url) => !url.includes("127.0.0.1") && !url.includes(".local")
      ) ??
      instance?.urls[0] ??
      "",
    [instance]
  );

  useEffect(() => {
    if (!shareURL) {
      setQrCode("");
      return;
    }
    void QRCode.toDataURL(shareURL, {
      width: 256,
      margin: 2,
      color: { dark: "#17221c", light: "#ffffff" }
    }).then(setQrCode);
  }, [shareURL]);

  async function changeState(action: "start" | "stop") {
    setWorking(true);
    setError("");
    try {
      const updated = action === "start" ? await startSite() : await stopSite();
      setData((current) => ({ ...current, instance: updated }));
      setAnnouncement(`Site ${action === "start" ? "started" : "stopped"}.`);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : `Unable to ${action} Site. Run spare doctor for details.`
      );
    } finally {
      setWorking(false);
      void refresh();
    }
  }

  return (
    <>
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <header className="masthead">
        <div className="brand" translate="no">
          <span className="brand-mark" aria-hidden="true">
            S
          </span>
          Spare
        </div>
        {data.machine && <p className="machine-name">{data.machine.hostname}</p>}
      </header>

      <main id="main" className="shell" aria-busy={loading}>
        <div role="status" className="sr-only">
          {announcement}
        </div>

        {error && (
          <section className="error-panel" role="alert">
            <strong>Spare needs attention</strong>
            <p>{error}</p>
          </section>
        )}

        {loading && !data.machine ? (
          <section className="hero" aria-label="Loading Spare">
            <p className="eyebrow">Checking this computer</p>
            <h1>Spare is starting</h1>
            <p className="lede">Reading the current job and local addresses.</p>
          </section>
        ) : instance ? (
          <SiteView
            instance={instance}
            machine={data.machine}
            qrCode={qrCode}
            shareURL={shareURL}
            working={working}
            onStart={() => void changeState("start")}
            onStop={() => void changeState("stop")}
          />
        ) : (
          <ReadyView machine={data.machine} />
        )}
      </main>
    </>
  );
}

function ReadyView({ machine }: { machine?: Machine }) {
  return (
    <>
      <section className="hero">
        <p className="eyebrow">
          <StatusDot tone="ready" /> Ready
        </p>
        <h1>This computer is ready</h1>
        <p className="lede">
          Give it a temporary job by serving a folder as a local website.
        </p>
      </section>

      <section className="card command-card" aria-labelledby="try-site-heading">
        <div>
          <p className="card-kicker">First job</p>
          <h2 id="try-site-heading">Try Site</h2>
          <p>
            Run this command from a folder that contains an{" "}
            <code translate="no">index.html</code> file.
          </p>
        </div>
        <code className="command" translate="no">
          spare try site ./public
        </code>
      </section>

      {machine && <MachineDetails machine={machine} />}
    </>
  );
}

function SiteView({
  instance,
  machine,
  qrCode,
  shareURL,
  working,
  onStart,
  onStop
}: {
  instance: Instance;
  machine?: Machine;
  qrCode: string;
  shareURL: string;
  working: boolean;
  onStart: () => void;
  onStop: () => void;
}) {
  const running = ["starting", "healthy", "degraded"].includes(instance.status);
  const tone = instance.status === "healthy" ? "healthy" : instance.status;

  return (
    <>
      <section className="hero">
        <p className={`eyebrow status-${tone}`}>
          <StatusDot tone={tone} /> {statusLabels[instance.status]}
        </p>
        <h1>This computer is a Site</h1>
        <p className="lede">
          {instance.status === "healthy"
            ? "Your folder is available on this computer and the local network."
            : instance.problem?.summary ?? "Spare is preparing your local website."}
        </p>
        {instance.problem && (
          <p className="recovery">{instance.problem.recovery}</p>
        )}
        <div className="actions" aria-label="Site controls">
          {instance.status === "healthy" && (
            <a
              className="button button-primary"
              href={instance.urls[0]}
              target="_blank"
              rel="noreferrer"
            >
              Open site
              <span className="sr-only"> in a new tab</span>
            </a>
          )}
          {!running && (
            <button
              className="button button-primary"
              type="button"
              onClick={onStart}
              disabled={working}
            >
              Start site
            </button>
          )}
          {running && (
            <button
              className="button button-secondary"
              type="button"
              onClick={onStop}
              disabled={working}
            >
              Stop site
            </button>
          )}
        </div>
      </section>

      <div className="content-grid">
        <section className="card address-card" aria-labelledby="addresses-heading">
          <div>
            <p className="card-kicker">Nearby devices</p>
            <h2 id="addresses-heading">Open Site</h2>
            <p>
              Use a LAN address while your devices are connected to the same
              network.
            </p>
          </div>
          <ul className="address-list">
            {instance.urls.map((url, index) => (
              <li key={url}>
                <span>{index === 0 ? "This computer" : url.includes(".local") ? "Local name" : "Local network"}</span>
                <a href={url} target="_blank" rel="noreferrer">
                  {url}
                  <span className="sr-only">, opens in a new tab</span>
                </a>
              </li>
            ))}
          </ul>
        </section>

        <section className="card qr-card" aria-labelledby="phone-heading">
          <div>
            <p className="card-kicker">Phone or tablet</p>
            <h2 id="phone-heading">Scan to open</h2>
          </div>
          {qrCode && (
            <img
              className="qr"
              src={qrCode}
              alt={`QR code for ${shareURL}`}
              width="192"
              height="192"
            />
          )}
          <a className="qr-address" href={shareURL}>
            {shareURL}
          </a>
        </section>
      </div>

      <details className="details-card">
        <summary>Show technical details</summary>
        <dl>
          <div>
            <dt>Mode</dt>
            <dd>{sentenceCase(instance.mode)}</dd>
          </div>
          <div>
            <dt>Folder</dt>
            <dd className="path" translate="no">
              {instance.rootPath}
            </dd>
          </div>
          <div>
            <dt>Port</dt>
            <dd>{instance.port}</dd>
          </div>
          {machine && (
            <>
              <div>
                <dt>System</dt>
                <dd>
                  {machine.os}/{machine.architecture}
                </dd>
              </div>
              <div>
                <dt>Memory</dt>
                <dd>{formatBytes(machine.memoryTotalBytes)}</dd>
              </div>
            </>
          )}
        </dl>
      </details>
    </>
  );
}

function MachineDetails({ machine }: { machine: Machine }) {
  return (
    <details className="details-card">
      <summary>Show computer details</summary>
      <dl>
        <div>
          <dt>Name</dt>
          <dd>{machine.hostname}</dd>
        </div>
        <div>
          <dt>System</dt>
          <dd>
            {machine.os}/{machine.architecture}
          </dd>
        </div>
        <div>
          <dt>CPU</dt>
          <dd>{machine.logicalCores} logical cores</dd>
        </div>
        <div>
          <dt>Memory</dt>
          <dd>{formatBytes(machine.memoryTotalBytes)}</dd>
        </div>
        <div>
          <dt>Available storage</dt>
          <dd>{formatBytes(machine.storageAvailableBytes)}</dd>
        </div>
      </dl>
    </details>
  );
}

function StatusDot({ tone }: { tone: string }) {
  return <span className={`status-dot status-dot-${tone}`} aria-hidden="true" />;
}

function formatBytes(value: number) {
  if (!value) return "Unavailable";
  const gibibytes = value / 1024 / 1024 / 1024;
  return `${gibibytes.toFixed(gibibytes >= 10 ? 0 : 1)} GB`;
}

function sentenceCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
