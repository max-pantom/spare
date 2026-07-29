import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import QRCode from "qrcode";
import { loadDashboard, startInstance, stopInstance } from "./api";
import { DesktopApp } from "./DesktopApp";
import { desktopBridge } from "./desktop";
import {
  createDashboardPreviewState,
  createDesktopPreviewBridge
} from "./desktopPreview";
import {
  displayMachineName,
  presentEvent,
  presentProblem
} from "./presentation";
import { SpareNavIcon, type SpareNavIconName } from "./SpareNavIcon";
import type { Event, Instance, Machine, Recipe } from "./types";

type DashboardState = {
  machine?: Machine;
  recipes: Recipe[];
  instances: Instance[];
  events: Event[];
};

const dashboardPages = [
  "overview",
  "transfer",
  "recipes",
  "machine",
  "activity"
] as const;

type DashboardPage = (typeof dashboardPages)[number];

const dashboardPageLabels: Record<DashboardPage, string> = {
  overview: "Dashboard",
  transfer: "Transfer",
  recipes: "Jobs",
  machine: "Computer",
  activity: "Activity"
};

const dashboardPageIcons: Record<DashboardPage, SpareNavIconName> = {
  overview: "home",
  transfer: "transfer",
  recipes: "jobs",
  machine: "computer",
  activity: "activity"
};

const statusLabels: Record<Instance["status"], string> = {
  starting: "Starting",
  healthy: "Ready",
  degraded: "Needs attention",
  stopped: "Stopped",
  failed: "Needs attention",
  removing: "Removing"
};

const previewParams = new URLSearchParams(window.location.search);

const desktopPreviewBridge =
  import.meta.env.DEV && previewParams.has("desktop-preview")
    ? createDesktopPreviewBridge()
    : undefined;

const dashboardPreview =
  import.meta.env.DEV && previewParams.has("dashboard-preview");

export function App() {
  const bridge = desktopBridge() ?? desktopPreviewBridge;
  if (bridge) {
    return <DesktopApp bridge={bridge} />;
  }
  return <BrowserDashboard />;
}

function BrowserDashboard() {
  const [data, setData] = useState<DashboardState>(() =>
    dashboardPreview
      ? createDashboardPreviewState()
      : {
          recipes: [],
          instances: [],
          events: []
        }
  );
  const [page, setPage] = useState<DashboardPage>(dashboardPageFromHash);
  const [loading, setLoading] = useState(!dashboardPreview);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [announcement, setAnnouncement] = useState("");
  const [qrCode, setQrCode] = useState("");
  const previousStatus = useRef<string | undefined>(undefined);

  const refresh = useCallback(async (announce = false) => {
    if (dashboardPreview) {
      setLoading(false);
      return;
    }
    try {
      const next = await loadDashboard();
      setData(next);
      setError("");
      const current = next.instances[0];
      const status = current ? `${current.id}:${current.status}` : "ready";
      if (
        announce &&
        previousStatus.current &&
        previousStatus.current !== status
      ) {
        const currentRecipe = next.recipes.find(
          (available) => available.id === current?.recipeId
        );
        setAnnouncement(
          current
            ? `${currentRecipe?.title ?? current.recipeId} status changed to ${statusLabels[current.status]}.`
            : `${next.machine?.hostname ?? "The Spare computer"} is ready for a recipe.`
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
    if (dashboardPreview) return;
    const timer = window.setInterval(() => void refresh(true), 2_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  useEffect(() => {
    const syncPage = () => setPage(dashboardPageFromHash());
    window.addEventListener("hashchange", syncPage);
    return () => window.removeEventListener("hashchange", syncPage);
  }, []);

  useEffect(() => {
    document.title = `${dashboardPageLabels[page]} · Spare`;
  }, [page]);

  const instance = data.instances[0];
  const activeRecipe = data.recipes.find(
    (available) => available.id === instance?.recipeId
  );
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
    if (!instance) return;
    const title = activeRecipe?.title ?? sentenceCase(instance.recipeId);
    setWorking(true);
    setError("");
    try {
      const updated: Instance = dashboardPreview
        ? {
            ...instance,
            desiredState: action === "start" ? "running" : "stopped",
            status: action === "start" ? "healthy" : "stopped",
            updatedAt: new Date().toISOString()
          }
        : action === "start"
          ? await startInstance(instance.id)
          : await stopInstance(instance.id);
      setData((current) => {
        const nextEvent: Event = {
          id: Math.max(0, ...current.events.map((event) => event.id)) + 1,
          instanceId: instance.id,
          level: "info",
          kind: action === "start" ? "instance_started" : "instance_stopped",
          message: `${title} ${action === "start" ? "started" : "stopped"}.`,
          createdAt: new Date().toISOString()
        };
        return {
          ...current,
          instances: [updated],
          events: dashboardPreview
            ? [nextEvent, ...current.events]
            : current.events
        };
      });
      setAnnouncement(
        `${title} ${action === "start" ? "started" : "stopped"}.`
      );
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : `Unable to ${action} ${title}. Open Spare on that computer and run repair.`
      );
    } finally {
      setWorking(false);
      if (!dashboardPreview) void refresh();
    }
  }

  return (
    <div className="dashboard-root">
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <aside className="dashboard-sidebar">
        <a className="dashboard-brand" href="#overview" translate="no">
          <span className="dashboard-brand-mark" aria-hidden="true" />
          <span>
            <strong>Spare</strong>
            <small>Browser dashboard</small>
          </span>
        </a>
        <nav className="dashboard-nav" aria-label="Dashboard sections">
          {dashboardPages.map((destination) => (
            <a
              key={destination}
              href={`#${destination}`}
              aria-current={page === destination ? "page" : undefined}
            >
              <SpareNavIcon
                className="dashboard-nav-icon"
                name={dashboardPageIcons[destination]}
                size={16}
              />
              {dashboardPageLabels[destination]}
            </a>
          ))}
        </nav>
        {data.machine && (
          <div className="dashboard-machine-summary">
            <StatusIcon
              tone={
                instance?.status === "healthy"
                  ? "healthy"
                  : instance?.status ?? "ready"
              }
            />
            <span>
              <strong>{data.machine.hostname}</strong>
              <small>
                {instance
                  ? `${activeRecipe?.title ?? sentenceCase(instance.recipeId)} · ${statusLabels[instance.status]}`
                  : "Ready for a recipe"}
              </small>
            </span>
          </div>
        )}
      </aside>

      <div className="dashboard-frame">
        <header className="dashboard-topbar">
          <div className="dashboard-remote-machine">
            <strong>
              {data.machine
                ? displayMachineName(data.machine.hostname)
                : "Spare computer"}
            </strong>
            <small className="dashboard-connection">
              <StatusIcon
                tone={error ? "failed" : loading ? "starting" : "healthy"}
              />
              {error
                ? "Connection needs attention"
                : loading
                  ? "Connecting locally"
                  : "Connected locally"}
            </small>
          </div>
          <p>{dashboardPageLabels[page]}</p>
          <details className="dashboard-mobile-menu">
            <summary>Menu</summary>
            <nav aria-label="Mobile dashboard sections">
              {dashboardPages.map((destination) => (
                <a
                  key={destination}
                  href={`#${destination}`}
                  aria-current={page === destination ? "page" : undefined}
                >
                  {dashboardPageLabels[destination]}
                </a>
              ))}
            </nav>
          </details>
        </header>

        <main id="main" className="dashboard-main" aria-busy={loading} tabIndex={-1}>
          <div role="status" className="sr-only">
            {announcement}
          </div>

          {error && (
            <section className="error-panel" role="alert">
              <strong>Unable to reach this computer</strong>
              <p>{error}</p>
              <p>Spare will keep trying to reconnect locally.</p>
            </section>
          )}

          {loading && !data.machine ? (
            <section className="dashboard-page hero" aria-label="Loading Spare">
              <p className="eyebrow">Checking this computer</p>
              <h1>Spare is starting</h1>
              <p className="lede">
                Reading its recipes, current job, and local addresses.
              </p>
            </section>
          ) : (
            <>
              {page === "overview" && (
                <DashboardOverview
                  instance={instance}
                  recipe={activeRecipe}
                  machine={data.machine}
                  events={data.events}
                  recipes={data.recipes}
                  shareURL={shareURL}
                />
              )}
              {page === "transfer" && (
                <InstanceView
                  instance={instance}
                  recipe={activeRecipe}
                  machine={data.machine}
                  qrCode={qrCode}
                  shareURL={shareURL}
                  working={working}
                  onStart={() => void changeState("start")}
                  onStop={() => void changeState("stop")}
                />
              )}
              {page === "recipes" && (
                <RecipesView
                  recipes={data.recipes}
                  installedRecipeID={instance?.recipeId}
                />
              )}
              {page === "machine" && data.machine && (
                <MachineView machine={data.machine} />
              )}
              {page === "activity" && (
                <ActivityView
                  events={data.events}
                  recipes={data.recipes}
                />
              )}
            </>
          )}
        </main>
      </div>
    </div>
  );
}

function DashboardOverview({
  instance,
  recipe,
  machine,
  events,
  recipes,
  shareURL
}: {
  instance?: Instance;
  recipe?: Recipe;
  machine?: Machine;
  events: Event[];
  recipes: Recipe[];
  shareURL: string;
}) {
  const title = recipe?.title ?? sentenceCase(instance?.recipeId ?? "recipe");
  const running = Boolean(
    instance && ["starting", "healthy", "degraded"].includes(instance.status)
  );
  const status = instance ? statusLabels[instance.status] : "Ready";

  return (
    <section
      id="overview"
      className="dashboard-page dashboard-overview"
      aria-labelledby="dashboard-heading"
    >
      <div className="dashboard-page-heading dashboard-overview-heading">
        <div>
          <p className="eyebrow">
            <StatusIcon
              tone={
                instance?.status === "healthy"
                  ? "healthy"
                  : instance?.status ?? "ready"
              }
            />
            Remote computer · {status}
          </p>
          <h1 id="dashboard-heading">
            {machine ? displayMachineName(machine.hostname) : "Spare computer"}
          </h1>
          <p className="lede">
            Access its current job, share access, recent activity, and basic
            controls from this browser.
          </p>
        </div>
        <a className="button button-primary" href="#transfer">
          Open current job
        </a>
      </div>

      <dl className="dashboard-stat-grid" aria-label="Dashboard summary">
        <div>
          <dt>Current job</dt>
          <dd>{instance ? title : "None"}</dd>
          <small>{running ? "Available now" : "Not running"}</small>
        </div>
        <div>
          <dt>
            {instance?.recipeId === "hook" ? "Requests received" : "Files received"}
          </dt>
          <dd>{instance?.itemCount ?? 0}</dd>
          <small>Since this role started</small>
        </div>
        <div>
          <dt>Available storage</dt>
          <dd>
            {formatBytes(
              instance?.storageAvailableBytes ??
                machine?.storageAvailableBytes ??
                0
            )}
          </dd>
          <small>On {machine?.hostname ?? "this computer"}</small>
        </div>
      </dl>

      <div className="dashboard-overview-grid">
        <section
          className="dashboard-panel dashboard-transfer-summary"
          aria-labelledby="transfer-summary-heading"
        >
          <div className="dashboard-panel-heading">
            <div>
              <p className="card-kicker">Transfer</p>
              <h2 id="transfer-summary-heading">
                {instance
                  ? `${title} is ${running ? "available" : "paused"}`
                  : "No transfer role is active"}
              </h2>
            </div>
            <span className={`dashboard-state status-${instance?.status ?? "ready"}`}>
              <StatusIcon
                tone={
                  instance?.status === "healthy"
                    ? "healthy"
                    : instance?.status ?? "ready"
                }
              />
              {status}
            </span>
          </div>
          <p>
            {instance
              ? remoteReadyMessage(instance, machine)
              : "Choose Drop, Site, or Hook on the Spare computer to make a local service available."}
          </p>
          {shareURL && (
            <code className="dashboard-address" translate="no">
              {shareURL}
            </code>
          )}
          <a className="dashboard-text-link" href="#transfer">
            Open transfer details <span aria-hidden="true">→</span>
          </a>
        </section>

        <section
          className="dashboard-panel dashboard-shortcuts"
          aria-labelledby="shortcuts-heading"
        >
          <div className="dashboard-panel-heading">
            <div>
              <p className="card-kicker">Manage</p>
              <h2 id="shortcuts-heading">Explore this computer</h2>
            </div>
          </div>
          <nav aria-label="Dashboard shortcuts">
            <a href="#recipes">
              <span>
                <strong>Jobs</strong>
                <small>Review Drop, Site, and Hook</small>
              </span>
              <span aria-hidden="true">→</span>
            </a>
            <a href="#machine">
              <span>
                <strong>Computer</strong>
                <small>See capabilities and system details</small>
              </span>
              <span aria-hidden="true">→</span>
            </a>
          </nav>
        </section>
      </div>

      <section
        className="dashboard-panel dashboard-recent"
        aria-labelledby="recent-heading"
      >
        <div className="dashboard-panel-heading">
          <div>
            <p className="card-kicker">Latest updates</p>
            <h2 id="recent-heading">Recent activity</h2>
          </div>
          <a className="dashboard-text-link" href="#activity">
            View all
          </a>
        </div>
        {events.length ? (
          <ol className="dashboard-recent-list">
            {events.slice(0, 4).map((event) => {
              const presented = presentEvent(
                event,
                event.instanceId
                  ? titleForRecipe(recipes, event.instanceId)
                  : "Spare"
              );
              return (
                <li key={event.id}>
                  <StatusIcon
                    tone={
                      event.level === "error"
                        ? "failed"
                        : event.level === "warning"
                          ? "degraded"
                          : "healthy"
                    }
                  />
                  <span>
                    <strong>{presented.summary}</strong>
                    <small>{formatTime(event.createdAt)}</small>
                  </span>
                </li>
              );
            })}
          </ol>
        ) : (
          <p className="empty-copy">
            No activity yet. Changes will appear here as they happen.
          </p>
        )}
      </section>
    </section>
  );
}

function InstanceView({
  instance,
  recipe,
  machine,
  qrCode,
  shareURL,
  working,
  onStart,
  onStop
}: {
  instance?: Instance;
  recipe?: Recipe;
  machine?: Machine;
  qrCode: string;
  shareURL: string;
  working: boolean;
  onStart: () => void;
  onStop: () => void;
}) {
  const [detailsOpen, setDetailsOpen] = useState(false);

  if (!instance) {
    return (
      <section
        id="transfer"
        className="dashboard-page view instance-view dashboard-transfer-page"
      >
        <div className="hero">
          <p className="eyebrow">
            <StatusIcon tone="ready" /> Ready
          </p>
          <h1>{machine?.hostname ?? "This Spare computer"} is ready</h1>
          <p className="lede">
            Use the Spare desktop app or CLI on that computer to choose Site,
            Drop, or Hook.
          </p>
        </div>
        <div className="empty-actions" aria-label="Try a recipe">
          <CommandCard
            kicker="Local website"
            title="Try Site"
            description="Serve a folder read-only without installing a permanent role."
            command="spare try site ./public"
          />
          <CommandCard
            kicker="File transfer"
            title="Try Drop"
            description="Receive files into a folder you select."
            command="spare try drop ./received-files"
          />
          <CommandCard
            kicker="Developer tools"
            title="Try Hook"
            description="Receive, inspect, and replay webhook requests."
            command="spare try hook"
          />
        </div>
      </section>
    );
  }

  const title = recipe?.title ?? sentenceCase(instance.recipeId);
  const running =
    instance.desiredState === "running" && instance.status !== "stopped";
  const tone = instance.status === "healthy" ? "healthy" : instance.status;
  const machineName = machine
    ? displayMachineName(machine.hostname)
    : "this Spare computer";
  const failure = instance.problem
    ? presentProblem(instance, title)
    : undefined;

  return (
    <section
      id="transfer"
      className="dashboard-page view instance-view dashboard-transfer-page"
    >
      <div className="hero">
        <p className={`eyebrow status-${tone}`}>
          <span className="dashboard-desktop-copy">
            <StatusIcon tone={tone} /> {statusLabels[instance.status]}
          </span>
          <span className="dashboard-mobile-copy">{title}</span>
        </p>
        <h1>
          <span className="dashboard-desktop-copy">
            {machineName} is a {title}
          </span>
          <span className="dashboard-mobile-copy">
            {instance.recipeId === "drop"
              ? `Send files to ${machineName}`
              : `Open ${title} on ${machineName}`}
          </span>
        </h1>
        {failure ? (
          <section
            className="dashboard-job-failure"
            aria-labelledby="dashboard-job-failure-heading"
          >
            <h2 id="dashboard-job-failure-heading">{failure.title}</h2>
            <p>{failure.explanation}</p>
          </section>
        ) : (
          <p className="lede">
            {instance.status === "healthy"
              ? remoteReadyMessage(instance, machine)
              : `Spare is preparing ${title} for this computer and the local network.`}
          </p>
        )}
        <div className="actions" aria-label={`${title} controls`}>
          {failure ? (
            <>
              <button
                className="button button-primary"
                type="button"
                onClick={onStart}
                disabled={working}
              >
                Run repair
              </button>
              <button
                className="button button-secondary"
                type="button"
                onClick={() => {
                  setDetailsOpen(true);
                  window.requestAnimationFrame(() => {
                    document
                      .getElementById("instance-details")
                      ?.scrollIntoView({ block: "nearest" });
                  });
                }}
              >
                View details
              </button>
              <button
                className="button button-secondary"
                type="button"
                onClick={onStop}
                disabled={working}
              >
                Stop {title}
              </button>
            </>
          ) : (
            <>
              {instance.status === "healthy" && (
                <a
                  className="button button-primary"
                  href={shareURL || instance.urls[0]}
                  target="_blank"
                  rel="noreferrer"
                >
                  <span className="dashboard-desktop-copy">Open {title}</span>
                  <span className="dashboard-mobile-copy">
                    {instance.recipeId === "drop"
                      ? "Choose files"
                      : `Open ${title}`}
                  </span>
                  <span className="sr-only"> in a new tab</span>
                </a>
              )}
              {!running ? (
                <button
                  className="button button-primary"
                  type="button"
                  onClick={onStart}
                  disabled={working}
                >
                  Start {title}
                </button>
              ) : (
                <button
                  className="button button-secondary"
                  type="button"
                  onClick={onStop}
                  disabled={working}
                >
                  Stop {title}
                </button>
              )}
            </>
          )}
        </div>
        <p className="remote-notice">
          You are accessing {machineName} from another device. Folder
          selection, installation, and removal stay on that computer.
        </p>
      </div>

      {instance.recipeId === "drop" && (
        <dl className="metric-row" aria-label="Drop storage and files">
          <div>
            <dt>Files received</dt>
            <dd>{instance.itemCount}</dd>
          </div>
          <div>
            <dt>Storage available</dt>
            <dd>{formatBytes(instance.storageAvailableBytes)}</dd>
          </div>
        </dl>
      )}
      {instance.recipeId === "hook" && (
        <dl className="metric-row metric-row-single" aria-label="Hook requests">
          <div>
            <dt>Requests received</dt>
            <dd>{instance.itemCount}</dd>
          </div>
        </dl>
      )}

      <div className="content-grid">
        <section className="card address-card" aria-labelledby="addresses-heading">
          <div>
            <p className="card-kicker">Nearby devices</p>
            <h2 id="addresses-heading">Open {title}</h2>
            <p>
              Use a LAN address while your devices are connected to the same
              network.
            </p>
          </div>
          <ul className="address-list">
            {instance.urls.map((url, index) => (
              <li key={url}>
                <span>
                  {index === 0
                    ? "This computer"
                    : url.includes(".local")
                      ? "Local name"
                      : "Local network"}
                </span>
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
          {shareURL && (
            <a className="qr-address" href={shareURL}>
              {shareURL}
            </a>
          )}
        </section>
      </div>

      <details
        id="instance-details"
        className="details-card"
        open={detailsOpen}
        onToggle={(event) => setDetailsOpen(event.currentTarget.open)}
      >
        <summary>Show instance details</summary>
        <dl>
          <Detail label="Recipe" value={`${title} ${instance.version}`} />
          <Detail label="Mode" value={sentenceCase(instance.mode)} />
          <Detail label="Runtime" value={sentenceCase(instance.runtime)} />
          {instance.dataPath && (
            <Detail label="Folder" value={instance.dataPath} path />
          )}
          <Detail label="Port" value={String(instance.port)} technical />
          {machine && (
            <Detail
              label="System"
              value={`${machine.os}/${machine.architecture}`}
            />
          )}
        </dl>
      </details>
    </section>
  );
}

function RecipesView({
  recipes,
  installedRecipeID
}: {
  recipes: Recipe[];
  installedRecipeID?: string;
}) {
  return (
    <section
      id="recipes"
      className="dashboard-page view dashboard-recipes-page"
      aria-labelledby="recipes-heading"
    >
      <div className="section-heading">
        <p className="eyebrow">Built in and trusted</p>
        <h1 id="recipes-heading">Jobs</h1>
        <p>
          Jobs describe useful outcomes. Installation and folder selection
          stay in the CLI for this preview.
        </p>
      </div>
      <div className="recipe-grid">
        {recipes.map((available) => {
          const installed = installedRecipeID === available.id;
          return (
            <article className="card recipe-card" key={available.id}>
              <div>
                <div className="recipe-title-row">
                  <h3>{available.title}</h3>
                  <span className="status-label">
                    <StatusIcon
                      tone={
                        installed
                          ? "healthy"
                          : available.compatibility.supported
                            ? "ready"
                            : "failed"
                      }
                    />
                    {installed
                      ? "Installed"
                      : available.compatibility.rating}
                  </span>
                </div>
                <p>{available.description}</p>
                {available.compatibility.warnings.map((warning) => (
                  <p className="inline-warning" key={warning}>
                    <span aria-hidden="true">△</span> {warning}
                  </p>
                ))}
              </div>
              <code className="command" translate="no">
                {tryCommand(available.id)}
              </code>
              <details className="inline-details">
                <summary>Review permissions</summary>
                <ul className="permission-list">
                  {available.permissions.map((permission) => (
                    <li key={permission.id}>
                      <span aria-hidden="true">
                        {permission.granted ? "✓" : "—"}
                      </span>
                      <span>
                        {permission.description}
                        {!permission.granted && " — not allowed"}
                      </span>
                    </li>
                  ))}
                </ul>
              </details>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function MachineView({ machine }: { machine: Machine }) {
  const capabilities = [
    ["Local network services", machine.capabilities.canServeLAN],
    ["Persistent recipes", machine.capabilities.canRunPersistent],
    ["Large file storage", machine.capabilities.canStoreLargeFiles],
    ["Container recipes", machine.capabilities.canRunContainers],
    ["External storage detected", machine.capabilities.hasExternalStorage]
  ] as const;
  return (
    <section
      id="machine"
      className="dashboard-page view dashboard-machine-page"
      aria-labelledby="machine-heading"
    >
      <div className="section-heading">
        <p className="eyebrow">Capability profile</p>
        <h1 id="machine-heading">Computer</h1>
        <p>
          Spare uses this profile to explain which recipes suit this computer.
        </p>
      </div>
      <section
        className="machine-details-section"
        aria-labelledby="machine-details-heading"
      >
        <div className="machine-section-heading">
          <p className="card-kicker">System profile</p>
          <h2 id="machine-details-heading">Computer details</h2>
        </div>
        <dl className="machine-detail-grid">
          <Detail label="Name" value={machine.hostname} />
          <Detail label="System" value={machine.os} />
          <Detail label="Architecture" value={machine.architecture} />
          <Detail
            label="CPU"
            value={`${machine.logicalCores} logical cores`}
          />
          <Detail label="Memory" value={formatBytes(machine.memoryTotalBytes)} />
          <Detail
            label="Available system storage"
            value={formatBytes(machine.storageAvailableBytes)}
          />
          <Detail
            label="Network"
            value={machine.lanAddresses.join(", ") || "Unavailable"}
            technical
          />
          <Detail
            label="Battery"
            value={
              machine.capabilities.hasBattery
                ? "Battery powered"
                : "No battery"
            }
          />
          <Detail
            label="External drives"
            value={
              machine.capabilities.hasExternalStorage
                ? "Detected"
                : "None detected"
            }
          />
          <Detail
            label="Container support"
            value={
              machine.capabilities.canRunContainers
                ? "Available"
                : "Unavailable"
            }
          />
        </dl>
      </section>
      <section
        className="card machine-card"
        aria-labelledby="machine-capabilities-heading"
      >
        <div className="machine-section-heading">
          <p className="card-kicker">Recipe support</p>
          <h2 id="machine-capabilities-heading">Capabilities</h2>
        </div>
        <ul className="capability-list">
          {capabilities.map(([label, available]) => (
            <li key={label}>
              <span aria-hidden="true">{available ? "✓" : "—"}</span>
              <span>{label}</span>
              <strong>{available ? "Available" : "Unavailable"}</strong>
            </li>
          ))}
        </ul>
      {machine.capabilities.hasBattery && (
        <p className="inline-warning">
          <span aria-hidden="true">△</span> This computer may sleep when its
          lid is closed.
        </p>
      )}
      </section>
    </section>
  );
}

function ActivityView({
  events,
  recipes
}: {
  events: Event[];
  recipes: Recipe[];
}) {
  return (
    <section
      id="activity"
      className="dashboard-page view dashboard-activity-page"
      aria-labelledby="activity-heading"
    >
      <div className="section-heading">
        <p className="eyebrow">System history</p>
        <h1 id="activity-heading">Activity</h1>
        <p>A human-readable history of what this computer has done.</p>
      </div>
      <div className="card activity-card">
        {events.length ? (
          <ol className="activity-list">
            {events.map((event) => {
              const presented = presentEvent(
                event,
                event.instanceId
                  ? titleForRecipe(recipes, event.instanceId)
                  : "Spare"
              );
              return (
                <li key={event.id}>
                  <StatusIcon
                    tone={
                      event.level === "error"
                        ? "failed"
                        : event.level === "warning"
                          ? "degraded"
                          : "healthy"
                    }
                  />
                  <div className="activity-row-content">
                    <div className="activity-message">
                      <strong>{presented.summary}</strong>
                      {presented.detail && <p>{presented.detail}</p>}
                    </div>
                    <time dateTime={event.createdAt}>
                      {formatTime(event.createdAt)}
                    </time>
                  </div>
                </li>
              );
            })}
          </ol>
        ) : (
          <p className="empty-copy">
            No activity yet. Start a recipe to see lifecycle events here.
          </p>
        )}
      </div>
    </section>
  );
}

function CommandCard({
  kicker,
  title,
  description,
  command
}: {
  kicker: string;
  title: string;
  description: string;
  command: string;
}) {
  const headingID = `${title.toLowerCase().replaceAll(" ", "-")}-heading`;
  return (
    <section className="card command-card" aria-labelledby={headingID}>
      <div>
        <p className="card-kicker">{kicker}</p>
        <h2 id={headingID}>{title}</h2>
        <p>{description}</p>
      </div>
      <code className="command" translate="no">
        {command}
      </code>
    </section>
  );
}

function Detail({
  label,
  value,
  path = false,
  technical = false
}: {
  label: string;
  value: string;
  path?: boolean;
  technical?: boolean;
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd
        className={path || technical ? "technical-value" : undefined}
        translate={path || technical ? "no" : undefined}
      >
        {value}
      </dd>
    </div>
  );
}

function StatusIcon({ tone }: { tone: string }) {
  return <span className={`status-dot status-dot-${tone}`} aria-hidden="true" />;
}

function remoteReadyMessage(instance: Instance, machine?: Machine) {
  const host = machine?.hostname ?? "the Spare computer";
  switch (instance.recipeId) {
    case "drop":
      return `Nearby devices can send files directly to ${displayMachineName(host)}.`;
    case "hook":
      return `Hook on ${host} is ready to receive, inspect, and replay webhook requests.`;
    default:
      return `The selected folder on ${host} is available as a read-only local website.`;
  }
}

function tryCommand(recipeID: string) {
  switch (recipeID) {
    case "drop":
      return "spare try drop ./received-files";
    case "hook":
      return "spare try hook";
    default:
      return "spare try site ./public";
  }
}

function titleForRecipe(recipes: Recipe[], id: string) {
  return (
    recipes.find((available) => available.id === id)?.title ?? sentenceCase(id)
  );
}

function formatBytes(value: number) {
  if (!value) return "Unavailable";
  const gibibytes = value / 1024 / 1024 / 1024;
  return `${gibibytes.toFixed(gibibytes >= 10 ? 0 : 1)} GB`;
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "Time unavailable";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(date);
}

function sentenceCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function dashboardPageFromHash(): DashboardPage {
  const candidate = window.location.hash.replace("#", "") as DashboardPage;
  return dashboardPages.includes(candidate) ? candidate : "overview";
}
