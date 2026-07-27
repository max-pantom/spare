import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import QRCode from "qrcode";
import { loadDashboard, startInstance, stopInstance } from "./api";
import type { Event, Instance, Machine, Recipe } from "./types";

type DashboardState = {
  machine?: Machine;
  recipes: Recipe[];
  instances: Instance[];
  events: Event[];
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
  const [data, setData] = useState<DashboardState>({
    recipes: [],
    instances: [],
    events: []
  });
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
            : "This computer is ready for a recipe."
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
      const updated =
        action === "start"
          ? await startInstance(instance.id)
          : await stopInstance(instance.id);
      setData((current) => ({
        ...current,
        instances: [updated]
      }));
      setAnnouncement(
        `${title} ${action === "start" ? "started" : "stopped"}.`
      );
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : `Unable to ${action} ${title}. Run spare doctor for details.`
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
        <div className="masthead-row">
          <div className="brand" translate="no">
            <span className="brand-mark" aria-hidden="true">
              S
            </span>
            Spare
          </div>
          {data.machine && (
            <p className="machine-name">{data.machine.hostname}</p>
          )}
        </div>
        <nav className="section-nav" aria-label="Dashboard sections">
          <a href="#machine">Machine</a>
          <a href="#recipes">Recipes</a>
          <a href="#instance">Instance</a>
          <a href="#activity">Activity</a>
        </nav>
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
            <p className="lede">
              Reading its recipes, current job, and local addresses.
            </p>
          </section>
        ) : (
          <>
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
            <RecipesView
              recipes={data.recipes}
              installedRecipeID={instance?.recipeId}
            />
            {data.machine && <MachineView machine={data.machine} />}
            <ActivityView events={data.events} recipes={data.recipes} />
          </>
        )}
      </main>
    </>
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
  if (!instance) {
    return (
      <section id="instance" className="view instance-view">
        <div className="hero">
          <p className="eyebrow">
            <StatusIcon tone="ready" /> Ready
          </p>
          <h1>This computer is ready</h1>
          <p className="lede">
            Choose Site to share a local website, Drop to receive files, or
            Hook to inspect webhooks from nearby devices.
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
  const running = ["starting", "healthy", "degraded"].includes(instance.status);
  const tone = instance.status === "healthy" ? "healthy" : instance.status;

  return (
    <section id="instance" className="view instance-view">
      <div className="hero">
        <p className={`eyebrow status-${tone}`}>
          <StatusIcon tone={tone} /> {statusLabels[instance.status]}
        </p>
        <h1>This computer is a {title}</h1>
        <p className="lede">
          {instance.status === "healthy"
            ? readyMessage(instance.recipeId)
            : instance.problem?.summary ??
              `Spare is preparing ${title} for this computer and the local network.`}
        </p>
        {instance.problem && (
          <p className="recovery">{instance.problem.recovery}</p>
        )}
        <div className="actions" aria-label={`${title} controls`}>
          {instance.status === "healthy" && (
            <a
              className="button button-primary"
              href={instance.urls[0]}
              target="_blank"
              rel="noreferrer"
            >
              Open {title}
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
              Start {title}
            </button>
          )}
          {running && (
            <button
              className="button button-secondary"
              type="button"
              onClick={onStop}
              disabled={working}
            >
              Stop {title}
            </button>
          )}
        </div>
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

      <details className="details-card">
        <summary>Show instance details</summary>
        <dl>
          <Detail label="Recipe" value={`${title} ${instance.version}`} />
          <Detail label="Mode" value={sentenceCase(instance.mode)} />
          <Detail label="Runtime" value={sentenceCase(instance.runtime)} />
          {instance.dataPath && (
            <Detail label="Folder" value={instance.dataPath} path />
          )}
          <Detail label="Port" value={String(instance.port)} />
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
    <section id="recipes" className="view" aria-labelledby="recipes-heading">
      <div className="section-heading">
        <p className="eyebrow">Built in and trusted</p>
        <h2 id="recipes-heading">Recipes</h2>
        <p>
          Recipes describe useful outcomes. Installation and folder selection
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
    <section id="machine" className="view" aria-labelledby="machine-heading">
      <div className="section-heading">
        <p className="eyebrow">Capability profile</p>
        <h2 id="machine-heading">Machine</h2>
        <p>
          Spare uses this profile to explain which recipes suit this computer.
        </p>
      </div>
      <div className="card machine-card">
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
      </div>
      <details className="details-card">
        <summary>Show computer details</summary>
        <dl>
          <Detail label="Name" value={machine.hostname} />
          <Detail
            label="System"
            value={`${machine.os}/${machine.architecture}`}
          />
          <Detail
            label="CPU"
            value={`${machine.logicalCores} logical cores`}
          />
          <Detail label="Memory" value={formatBytes(machine.memoryTotalBytes)} />
          <Detail
            label="Available system storage"
            value={formatBytes(machine.storageAvailableBytes)}
          />
        </dl>
      </details>
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
    <section id="activity" className="view" aria-labelledby="activity-heading">
      <div className="section-heading">
        <p className="eyebrow">Recent changes</p>
        <h2 id="activity-heading">Activity</h2>
        <p>Starts, stops, failures, recovery, and port changes appear here.</p>
      </div>
      <div className="card activity-card">
        {events.length ? (
          <ol className="activity-list">
            {events.map((event) => (
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
                <div>
                  <strong>
                    {event.instanceId
                      ? titleForRecipe(recipes, event.instanceId)
                      : "Spare"}
                  </strong>
                  <p>{event.message}</p>
                  <time dateTime={event.createdAt}>
                    {formatTime(event.createdAt)}
                  </time>
                </div>
              </li>
            ))}
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
  path = false
}: {
  label: string;
  value: string;
  path?: boolean;
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd className={path ? "path" : undefined} translate={path ? "no" : undefined}>
        {value}
      </dd>
    </div>
  );
}

function StatusIcon({ tone }: { tone: string }) {
  return <span className={`status-dot status-dot-${tone}`} aria-hidden="true" />;
}

function readyMessage(recipeID: string) {
  switch (recipeID) {
    case "drop":
      return "Nearby devices can upload and download files through your selected folder.";
    case "hook":
      return "Hook is ready to receive, inspect, and replay webhook requests.";
    default:
      return "Your folder is available as a read-only website on this computer and the local network.";
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
