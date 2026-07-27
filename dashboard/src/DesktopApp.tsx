import {
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useState
} from "react";
import QRCode from "qrcode";
import type { DesktopBridge } from "./desktop";
import type {
  DesktopPreferences,
  DesktopSnapshot,
  Event,
  Instance,
  Recipe
} from "./types";

type View = "home" | "recipes" | "activity" | "machine" | "settings";

const navigation: Array<{ id: View; label: string }> = [
  { id: "home", label: "Home" },
  { id: "recipes", label: "Recipes" },
  { id: "activity", label: "Activity" },
  { id: "machine", label: "Machine" },
  { id: "settings", label: "Settings" }
];

const statusLabels: Record<Instance["status"], string> = {
  starting: "Starting",
  healthy: "Ready",
  degraded: "Needs attention",
  stopped: "Stopped",
  failed: "Needs attention",
  removing: "Removing"
};

export function DesktopApp({ bridge }: { bridge: DesktopBridge }) {
  const [snapshot, setSnapshot] = useState<DesktopSnapshot>();
  const [view, setView] = useState<View>("home");
  const [setupRecipe, setSetupRecipe] = useState<Recipe>();
  const [editingInstance, setEditingInstance] = useState<Instance>();
  const [setupInitialValues, setSetupInitialValues] = useState<
    Record<string, unknown>
  >({});
  const [droppedBackup, setDroppedBackup] = useState("");
  const [showShare, setShowShare] = useState(false);
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [announcement, setAnnouncement] = useState("");

  const refresh = useCallback(async () => {
    try {
      const next = await bridge.Snapshot();
      setSnapshot(next);
      setError("");
    } catch (requestError) {
      setError(errorMessage(requestError, "Unable to read Spare."));
    }
  }, [bridge]);

  const processDroppedPaths = useCallback(
    async (paths: string[]) => {
      if (!paths.length) return;
      try {
        const dropped = await bridge.DescribeDroppedPaths(paths);
        const current = await bridge.Snapshot();
        const currentInstance = current.instances[0];
        if (dropped.length === 1 && dropped[0].kind === "recipe-package") {
          await bridge.OpenRecipePackage(dropped[0].path);
          setAnnouncement(`${dropped[0].name} opened in the safe recipe viewer.`);
          return;
        }
        if (dropped.length === 1 && dropped[0].kind === "backup") {
          setDroppedBackup(dropped[0].path);
          setView("settings");
          setAnnouncement(`${dropped[0].name} is ready to restore.`);
          return;
        }
        if (dropped.length === 1 && dropped[0].kind === "directory") {
          if (currentInstance) {
            throw new Error(
              "Remove the current job before setting up Site with another folder."
            );
          }
          const site = current.recipes.find((recipe) => recipe.id === "site");
          if (!site) throw new Error("Site is unavailable in this Spare build.");
          const pathField =
            site.config.find((field) => field.type === "directory")?.id ?? "path";
          setSetupInitialValues({ [pathField]: dropped[0].path });
          setEditingInstance(undefined);
          setSetupRecipe(site);
          setView("recipes");
          setAnnouncement(`${dropped[0].name} selected for Site.`);
          return;
        }
        if (
          dropped.every((item) => item.kind === "file") &&
          currentInstance?.recipeId === "drop"
        ) {
          const names = await bridge.AddDropFiles(
            currentInstance.id,
            dropped.map((item) => item.path)
          );
          setAnnouncement(
            `${names.length} ${names.length === 1 ? "file was" : "files were"} added to Drop.`
          );
          await refresh();
          return;
        }
        throw new Error(
          "Drop files onto an active Drop, a folder onto Site setup, or open one .sp or .spare-backup file."
        );
      } catch (requestError) {
        setError(errorMessage(requestError, "Spare could not use the dropped item."));
      }
    },
    [bridge, refresh]
  );

  useEffect(() => {
    let active = true;
    void bridge
      .Bootstrap()
      .then((next) => {
        if (active) {
          setSnapshot(next);
          setError("");
          void bridge.PendingLaunchPaths().then((paths) => {
            if (active) void processDroppedPaths(paths);
          });
        }
      })
      .catch((requestError) => {
        if (active) {
          setError(
            errorMessage(
              requestError,
              "Spare could not start its background service."
            )
          );
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    const timer = window.setInterval(() => void refresh(), 2_000);
    const stopActivity = window.runtime?.EventsOn?.(
      "spare:activity",
      () => void refresh()
    );
    const stopNavigation = window.runtime?.EventsOn?.(
      "spare:navigate",
      (destination) => {
        if (
          destination === "activity" ||
          destination === "recipes" ||
          destination === "share"
        ) {
          if (destination === "share") {
            setView("home");
            setShowShare(true);
          } else {
            setView(destination);
          }
        }
      }
    );
    const stopFileDrop = window.runtime?.EventsOn?.(
      "spare:file-drop",
      (paths) => {
        if (Array.isArray(paths)) {
          void processDroppedPaths(
            paths.filter((path): path is string => typeof path === "string")
          );
        }
      }
    );
    return () => {
      active = false;
      window.clearInterval(timer);
      stopActivity?.();
      stopNavigation?.();
      stopFileDrop?.();
    };
  }, [bridge, processDroppedPaths, refresh]);

  const instance = snapshot?.instances[0];
  const activeRecipe = snapshot?.recipes.find(
    (recipe) => recipe.id === instance?.recipeId
  );

  async function lifecycle(action: "start" | "stop") {
    if (!instance) return;
    const title = activeRecipe?.title ?? instance.recipeId;
    setWorking(true);
    setError("");
    try {
      const updated =
        action === "start"
          ? await bridge.StartInstance(instance.id)
          : await bridge.StopInstance(instance.id);
      setSnapshot((current) =>
        current ? { ...current, instances: [updated] } : current
      );
      setAnnouncement(
        `${title} ${action === "start" ? "started" : "stopped"}.`
      );
      await refresh();
    } catch (requestError) {
      setError(
        errorMessage(
          requestError,
          `Unable to ${action} ${title}. Run repair and try again.`
        )
      );
    } finally {
      setWorking(false);
    }
  }

  async function addFilesToDrop() {
    if (!instance || instance.recipeId !== "drop") return;
    setWorking(true);
    setError("");
    try {
      const paths = await bridge.ChooseFiles("drop");
      if (!paths.length) return;
      const names = await bridge.AddDropFiles(instance.id, paths);
      setAnnouncement(
        `${names.length} ${names.length === 1 ? "file was" : "files were"} added to Drop.`
      );
      await refresh();
    } catch (requestError) {
      setError(errorMessage(requestError, "Spare could not add those files to Drop."));
    } finally {
      setWorking(false);
    }
  }

  async function repair() {
    setWorking(true);
    setError("");
    try {
      const next = await bridge.Repair();
      setSnapshot(next);
      setAnnouncement("Spare repaired its connection to the background service.");
    } catch (requestError) {
      setError(
        errorMessage(
          requestError,
          "Repair could not start Spare. Open diagnostics for details."
        )
      );
    } finally {
      setWorking(false);
    }
  }

  function navigate(destination: View) {
    setView(destination);
    setSetupRecipe(undefined);
    setEditingInstance(undefined);
    setSetupInitialValues({});
    setShowShare(false);
    setError("");
  }

  return (
    <div className="desktop-root">
      <a className="skip-link" href="#desktop-main">
        Skip to content
      </a>
      <aside className="desktop-sidebar">
        <div className="desktop-brand" translate="no">
          <span className="brand-mark" aria-hidden="true">
            S
          </span>
          <span>
            <strong>Spare</strong>
            <small>{snapshot?.machine.hostname ?? "This computer"}</small>
          </span>
        </div>
        <nav className="desktop-nav" aria-label="Spare">
          {navigation.map((item) => (
            <button
              key={item.id}
              type="button"
              aria-current={view === item.id ? "page" : undefined}
              onClick={() => navigate(item.id)}
            >
              <NavIcon name={item.id} />
              {item.label}
            </button>
          ))}
        </nav>
        <div className="desktop-sidebar-status">
          <StatusIcon status={instance?.status} />
          <span>
            <strong>
              {instance
                ? `${activeRecipe?.title ?? instance.recipeId} ${statusLabels[
                    instance.status
                  ].toLowerCase()}`
                : "No active job"}
            </strong>
            <small>Background service connected</small>
          </span>
        </div>
      </aside>

      <main id="desktop-main" className="desktop-main" aria-busy={loading}>
        <div className="sr-only" role="status" aria-live="polite">
          {announcement}
        </div>
        {error && (
          <section className="error-panel desktop-error" role="alert">
            <strong>Spare needs attention</strong>
            <p>{error}</p>
            <button
              className="button button-secondary"
              type="button"
              onClick={() => void repair()}
              disabled={working}
            >
              Run repair
            </button>
          </section>
        )}

        {loading && !snapshot ? (
          <LoadingView />
        ) : setupRecipe && snapshot ? (
          <SetupView
            bridge={bridge}
            recipe={setupRecipe}
            initialValues={setupInitialValues}
            existingInstance={editingInstance}
            onBack={() => {
              setSetupRecipe(undefined);
              setEditingInstance(undefined);
            }}
            onStarted={async (created) => {
              const wasEditing = Boolean(editingInstance);
              setSetupRecipe(undefined);
              setEditingInstance(undefined);
              setView("home");
              setAnnouncement(
                wasEditing
                  ? `${setupRecipe.title} configuration was updated.`
                  : `${setupRecipe.title} ${created.mode === "temporary" ? "is running temporarily" : "will keep running after login"}.`
              );
              await refresh();
            }}
          />
        ) : snapshot ? (
          <>
            {view === "home" && (
              <HomeView
                bridge={bridge}
                snapshot={snapshot}
                instance={instance}
                recipe={activeRecipe}
                working={working}
                showShare={showShare}
                onShowShare={() => setShowShare((current) => !current)}
                onChoose={(recipe) => {
                  setSetupInitialValues({});
                  setEditingInstance(undefined);
                  setSetupRecipe(recipe);
                  setView("recipes");
                }}
                onStart={() => void lifecycle("start")}
                onStop={() => void lifecycle("stop")}
                onAddFiles={() => void addFilesToDrop()}
                onActivity={() => navigate("activity")}
                onConfigure={() => {
                  if (instance && activeRecipe) {
                    setSetupInitialValues(instance.config);
                    setEditingInstance(instance);
                    setSetupRecipe(activeRecipe);
                  }
                }}
                onRecipes={() => navigate("recipes")}
                onMachine={() => navigate("machine")}
              />
            )}
            {view === "recipes" && (
              <RecipesView
                bridge={bridge}
                snapshot={snapshot}
                onChoose={(recipe) => {
                  setSetupInitialValues({});
                  setEditingInstance(undefined);
                  setSetupRecipe(recipe);
                }}
                onChanged={refresh}
              />
            )}
            {view === "activity" && (
              <ActivityView
                bridge={bridge}
                events={snapshot.events}
                recipes={snapshot.recipes}
                instances={snapshot.instances}
              />
            )}
            {view === "machine" && <MachineView snapshot={snapshot} />}
            {view === "settings" && (
              <SettingsView
                bridge={bridge}
                snapshot={snapshot}
                preferences={snapshot.preferences}
                initialBackupPath={droppedBackup}
                onSaved={refresh}
                onRepair={() => void repair()}
                onConfigure={() => {
                  if (instance && activeRecipe) {
                    setSetupInitialValues(instance.config);
                    setEditingInstance(instance);
                    setSetupRecipe(activeRecipe);
                  }
                }}
                onRestored={async () => {
                  setDroppedBackup("");
                  setView("home");
                  await refresh();
                }}
              />
            )}
          </>
        ) : (
          <ServiceUnavailable onRepair={() => void repair()} working={working} />
        )}
      </main>
    </div>
  );
}

function LoadingView() {
  return (
    <section className="desktop-page desktop-loading">
      <p className="eyebrow">Preparing this computer</p>
      <h1>Spare is starting</h1>
      <p className="lede">
        Creating local state, starting the background service, and profiling
        this computer.
      </p>
    </section>
  );
}

function ServiceUnavailable({
  onRepair,
  working
}: {
  onRepair: () => void;
  working: boolean;
}) {
  return (
    <section className="desktop-page">
      <p className="eyebrow status-failed">
        <StatusIcon status="failed" /> Background service unavailable
      </p>
      <h1>Spare could not start</h1>
      <p className="lede">
        Repair the local service, then Spare will reconnect without changing
        any selected folders.
      </p>
      <div className="actions">
        <button
          className="button button-primary"
          type="button"
          onClick={onRepair}
          disabled={working}
        >
          Run repair
        </button>
      </div>
    </section>
  );
}

function HomeView({
  bridge,
  snapshot,
  instance,
  recipe,
  working,
  showShare,
  onShowShare,
  onChoose,
  onStart,
  onStop,
  onAddFiles,
  onActivity,
  onConfigure,
  onRecipes,
  onMachine
}: {
  bridge: DesktopBridge;
  snapshot: DesktopSnapshot;
  instance?: Instance;
  recipe?: Recipe;
  working: boolean;
  showShare: boolean;
  onShowShare: () => void;
  onChoose: (recipe: Recipe) => void;
  onStart: () => void;
  onStop: () => void;
  onAddFiles: () => void;
  onActivity: () => void;
  onConfigure: () => void;
  onRecipes: () => void;
  onMachine: () => void;
}) {
  const drop =
    snapshot.recipes.find((candidate) => candidate.id === "drop") ??
    snapshot.recipes[0];
  if (!instance) {
    return (
      <section className="desktop-page">
        <div className="desktop-hero">
          <p className="eyebrow">This computer is ready</p>
          <h1>Give this computer a job.</h1>
          <p className="lede">
            {snapshot.machine.hostname} can receive files, serve a folder, or
            inspect local webhooks.
          </p>
          {drop && (
            <div className="actions">
              <button
                className="button button-primary"
                type="button"
                onClick={() => onChoose(drop)}
              >
                Try Drop
              </button>
              <button
                className="button button-secondary"
                type="button"
                onClick={onRecipes}
              >
                Choose another job
              </button>
              <button
                className="button button-quiet"
                type="button"
                onClick={onMachine}
              >
                See what this computer can do
              </button>
            </div>
          )}
        </div>
        <div className="ready-grid" aria-label="Ready for">
          {snapshot.recipes.map((available) => (
            <button
              type="button"
              className="ready-card"
              key={available.id}
              onClick={() => onChoose(available)}
              disabled={!available.compatibility.supported}
            >
              <RecipeIcon recipe={available.id} />
              <span>
                <strong>{available.title}</strong>
                <small>{shortDescription(available.id)}</small>
              </span>
              <span aria-hidden="true">→</span>
            </button>
          ))}
        </div>
      </section>
    );
  }

  const title = recipe?.title ?? instance.recipeId;
  const running = ["starting", "healthy", "degraded"].includes(instance.status);
  const recent = snapshot.events.slice(0, 3);
  return (
    <section className="desktop-page">
      <div className="desktop-hero">
        <p className={`eyebrow status-${instance.status}`}>
          <StatusIcon status={instance.status} />
          {statusLabels[instance.status]}
        </p>
        <h1>This computer is a {title}</h1>
        <p className="lede">
          {instance.problem?.summary ??
            (instance.status === "healthy"
              ? readyDescription(instance.recipeId)
              : `${title} is ${statusLabels[instance.status].toLowerCase()}.`)}
        </p>
        {instance.problem && (
          <p className="recovery">{instance.problem.recovery}</p>
        )}
        <div className="actions" aria-label={`${title} controls`}>
          {instance.status === "healthy" && instance.urls[0] && (
            <button
              className="button button-primary"
              type="button"
              onClick={() => void bridge.OpenURL(instance.urls[0])}
            >
              Open {title}
            </button>
          )}
          {instance.status === "healthy" && (
            <button
              className="button button-secondary"
              type="button"
              onClick={onShowShare}
              aria-expanded={showShare}
            >
              {showShare ? "Hide access" : "Share access"}
            </button>
          )}
          {instance.recipeId === "drop" && (
            <button
              className="button button-secondary"
              type="button"
              onClick={onAddFiles}
              disabled={working}
            >
              Add files to Drop
            </button>
          )}
          {running ? (
            <button
              className="button button-secondary"
              type="button"
              onClick={onStop}
              disabled={working}
            >
              Stop {title}
            </button>
          ) : (
            <button
              className="button button-primary"
              type="button"
              onClick={onStart}
              disabled={working}
            >
              Start {title}
            </button>
          )}
          <button
            className="button button-quiet"
            type="button"
            onClick={onConfigure}
          >
            Configure
          </button>
        </div>
      </div>

      {instance.recipeId === "drop" && (
        <dl className="desktop-metrics" aria-label="Drop status">
          <div>
            <dt>Received</dt>
            <dd>
              {instance.itemCount} {instance.itemCount === 1 ? "file" : "files"}
            </dd>
          </div>
          <div>
            <dt>Storage</dt>
            <dd>{formatBytes(instance.storageAvailableBytes)} available</dd>
          </div>
          <div>
            <dt>Mode</dt>
            <dd>
              {instance.mode === "installed"
                ? "Runs after login"
                : "Temporary"}
            </dd>
          </div>
        </dl>
      )}

      {showShare && (
        <SharePanel instance={instance} bridge={bridge} title={title} />
      )}

      <section className="desktop-section" aria-labelledby="recent-heading">
        <div className="desktop-section-heading">
          <div>
            <p className="eyebrow">Latest changes</p>
            <h2 id="recent-heading">Recent activity</h2>
          </div>
          <button className="text-button" type="button" onClick={onActivity}>
            View activity
          </button>
        </div>
        <EventList
          bridge={bridge}
          events={recent}
          recipes={snapshot.recipes}
          instances={snapshot.instances}
        />
      </section>
    </section>
  );
}

function SharePanel({
  instance,
  bridge,
  title
}: {
  instance: Instance;
  bridge: DesktopBridge;
  title: string;
}) {
  const shareURL =
    instance.urls.find(
      (value) => !value.includes("127.0.0.1") && !value.includes(".local")
    ) ??
    instance.urls[0] ??
    "";
  const [qrCode, setQrCode] = useState("");
  useEffect(() => {
    if (!shareURL) return;
    void QRCode.toDataURL(shareURL, {
      width: 256,
      margin: 2,
      color: { dark: "#17221c", light: "#ffffff" }
    }).then(setQrCode);
  }, [shareURL]);
  return (
    <section className="desktop-share" aria-labelledby="share-heading">
      <div>
        <p className="eyebrow">Phone or nearby computer</p>
        <h2 id="share-heading">Open {title} nearby</h2>
        <p>
          Scan the code or enter the address while both devices use the same
          local network.
        </p>
        <button
          className="text-button"
          type="button"
          onClick={() => void bridge.OpenURL(shareURL)}
        >
          Open this address
        </button>
      </div>
      <div className="desktop-qr">
        {qrCode && (
          <img
            src={qrCode}
            alt={`QR code for ${shareURL}`}
            width="176"
            height="176"
          />
        )}
        <code>{shareURL}</code>
      </div>
    </section>
  );
}

function SetupView({
  bridge,
  recipe,
  initialValues,
  existingInstance,
  onBack,
  onStarted
}: {
  bridge: DesktopBridge;
  recipe: Recipe;
  initialValues: Record<string, unknown>;
  existingInstance?: Instance;
  onBack: () => void;
  onStarted: (instance: Instance) => void;
}) {
  const initial = useMemo(
    () =>
      ({
        ...Object.fromEntries(
          recipe.config
            .filter((field) => field.default !== undefined)
            .map((field) => [field.id, field.default])
        ),
        ...initialValues
      }),
    [initialValues, recipe]
  );
  const [values, setValues] =
    useState<Record<string, unknown>>(initial);
  const [mode, setMode] = useState<"temporary" | "installed">(
    existingInstance?.mode ?? "installed"
  );
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");

  async function chooseDirectory(fieldID: string) {
    try {
      const selected = await bridge.ChooseDirectory(recipe.id);
      if (selected) {
        setValues((current) => ({ ...current, [fieldID]: selected }));
        setError("");
      }
    } catch (requestError) {
      setError(
        errorMessage(requestError, "Unable to open the folder picker.")
      );
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    const missing = recipe.config.find(
      (field) => field.required && !String(values[field.id] ?? "").trim()
    );
    if (missing) {
      setError(`Choose ${missing.label.toLowerCase()} before starting ${recipe.title}.`);
      document.getElementById(`config-${missing.id}`)?.focus();
      return;
    }
    setWorking(true);
    setError("");
    try {
      const input = {
        recipeId: recipe.id,
        mode,
        config: values,
        port: existingInstance?.portMode === "fixed" ? existingInstance.port : 0,
        portMode: existingInstance?.portMode ?? ("auto" as const)
      };
      const created = existingInstance
        ? await bridge.ConfigureInstance(existingInstance.id, input)
        : await bridge.CreateInstance(input);
      onStarted(created);
    } catch (requestError) {
      setError(
        errorMessage(
          requestError,
          `Unable to start ${recipe.title}. Review its settings and try again.`
        )
      );
    } finally {
      setWorking(false);
    }
  }

  return (
    <section className="desktop-page setup-page">
      <button className="back-button" type="button" onClick={onBack}>
        <span aria-hidden="true">←</span> Back to recipes
      </button>
      <div className="desktop-page-heading">
        <p className="eyebrow">Built in and trusted</p>
        <h1>{existingInstance ? `Configure ${recipe.title}` : `Set up ${recipe.title}`}</h1>
        <p className="lede">{recipe.description}</p>
      </div>
      <form className="setup-form" onSubmit={(event) => void submit(event)}>
        {error && (
          <div className="form-error" role="alert">
            {error}
          </div>
        )}
        {recipe.config.map((field) => (
          <div className="setup-field" key={field.id}>
            <label id={`label-${field.id}`} htmlFor={`config-${field.id}`}>
              {field.label}
            </label>
            {field.description && <p>{field.description}</p>}
            {field.type === "directory" ? (
              <div className="folder-picker">
                <output
                  id={`config-${field.id}`}
                  aria-labelledby={`label-${field.id}`}
                  tabIndex={-1}
                >
                  {String(values[field.id] ?? "No folder selected")}
                </output>
                <button
                  className="button button-secondary"
                  type="button"
                  onClick={() => void chooseDirectory(field.id)}
                >
                  Choose folder
                </button>
              </div>
            ) : field.type === "boolean" ? (
              <input
                id={`config-${field.id}`}
                type="checkbox"
                checked={Boolean(values[field.id])}
                onChange={(event) =>
                  setValues((current) => ({
                    ...current,
                    [field.id]: event.target.checked
                  }))
                }
              />
            ) : (
              <input
                id={`config-${field.id}`}
                name={field.id}
                type={field.type === "integer" ? "number" : "text"}
                value={String(values[field.id] ?? "")}
                placeholder={field.type === "size" ? "2 GB" : undefined}
                onChange={(event) =>
                  setValues((current) => ({
                    ...current,
                    [field.id]: event.target.value
                  }))
                }
              />
            )}
          </div>
        ))}
        {!existingInstance && (
          <fieldset className="mode-picker">
            <legend>Run</legend>
            <label>
              <input
                type="radio"
                name="mode"
                value="installed"
                checked={mode === "installed"}
                onChange={() => setMode("installed")}
              />
              <span>
                <strong>Keep running after login</strong>
                <small>Spare restores this job when you sign in.</small>
              </span>
            </label>
            <label>
              <input
                type="radio"
                name="mode"
                value="temporary"
                checked={mode === "temporary"}
                onChange={() => setMode("temporary")}
              />
              <span>
                <strong>Try temporarily</strong>
                <small>Spare keeps this job while the desktop app runs.</small>
              </span>
            </label>
          </fieldset>
        )}
        <details className="permission-review">
          <summary>Review permissions</summary>
          <ul>
            {recipe.permissions.map((permission) => (
              <li key={permission.id}>
                <span aria-hidden="true">
                  {permission.granted ? "✓" : "—"}
                </span>
                {permission.description}
                {!permission.granted && " — not allowed"}
              </li>
            ))}
          </ul>
        </details>
        <div className="setup-actions">
          <button
            className="button button-primary"
            type="submit"
            disabled={working}
          >
            {existingInstance ? "Save configuration" : `Start ${recipe.title}`}
          </button>
          <button
            className="button button-secondary"
            type="button"
            onClick={onBack}
          >
            Cancel
          </button>
        </div>
      </form>
    </section>
  );
}

function RecipesView({
  bridge,
  snapshot,
  onChoose,
  onChanged
}: {
  bridge: DesktopBridge;
  snapshot: DesktopSnapshot;
  onChoose: (recipe: Recipe) => void;
  onChanged: () => Promise<void>;
}) {
  const instance = snapshot.instances[0];
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [error, setError] = useState("");
  async function remove() {
    if (!instance) return;
    try {
      await bridge.RemoveInstance(instance.id);
      setConfirmRemove(false);
      await onChanged();
    } catch (requestError) {
      setError(
        errorMessage(
          requestError,
          "Unable to remove this job. Its selected folder was not changed."
        )
      );
    }
  }
  return (
    <section className="desktop-page">
      <div className="desktop-page-heading">
        <p className="eyebrow">Built in and trusted</p>
        <h1>Recipes</h1>
        <p className="lede">
          Choose one useful job for this computer. You can change it without
          deleting any selected folder.
        </p>
        <button
          className="button button-quiet"
          type="button"
          onClick={() => void bridge.OpenRecipePackage("")}
        >
          Inspect a recipe package
        </button>
      </div>
      {error && (
        <div className="form-error" role="alert">
          {error}
        </div>
      )}
      {instance && (
        <section className="installed-recipe" aria-labelledby="installed-heading">
          <div>
            <p className="card-kicker">Installed</p>
            <h2 id="installed-heading">
              {titleForRecipe(snapshot.recipes, instance.recipeId)}
            </h2>
            <p className="path">{instance.dataPath || "No selected folder"}</p>
          </div>
          <div className="actions">
            {instance.dataPath && (
              <button
                className="button button-secondary"
                type="button"
                onClick={() => void bridge.RevealPath(instance.dataPath)}
              >
                Show folder
              </button>
            )}
            {!confirmRemove ? (
              <button
                className="button button-quiet"
                type="button"
                onClick={() => setConfirmRemove(true)}
              >
                Change job
              </button>
            ) : (
              <>
                <button
                  className="button button-danger"
                  type="button"
                  onClick={() => void remove()}
                >
                  Remove {titleForRecipe(snapshot.recipes, instance.recipeId)}
                </button>
                <button
                  className="button button-secondary"
                  type="button"
                  onClick={() => setConfirmRemove(false)}
                >
                  Cancel
                </button>
              </>
            )}
          </div>
          {confirmRemove && (
            <p className="remove-note" role="status">
              Spare will remove this job and its logs. The selected folder and
              every file inside it will remain unchanged.
            </p>
          )}
        </section>
      )}
      <div className="desktop-recipe-grid" aria-label="Available recipes">
        {snapshot.recipes.map((recipe) => (
          <article className="desktop-recipe-card" key={recipe.id}>
            <RecipeIcon recipe={recipe.id} />
            <div>
              <div className="recipe-title-row">
                <h2>{recipe.title}</h2>
                <span className="status-label">
                  <StatusIcon
                    status={
                      instance?.recipeId === recipe.id
                        ? instance.status
                        : recipe.compatibility.supported
                          ? "healthy"
                          : "failed"
                    }
                  />
                  {instance?.recipeId === recipe.id
                    ? "Active"
                    : recipe.compatibility.rating}
                </span>
              </div>
              <p>{recipe.description}</p>
            </div>
            <button
              className="button button-secondary"
              type="button"
              onClick={() => onChoose(recipe)}
              disabled={Boolean(instance) || !recipe.compatibility.supported}
            >
              Set up {recipe.title}
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}

function ActivityView({
  bridge,
  events,
  recipes,
  instances
}: {
  bridge: DesktopBridge;
  events: Event[];
  recipes: Recipe[];
  instances: Instance[];
}) {
  return (
    <section className="desktop-page">
      <div className="desktop-page-heading">
        <p className="eyebrow">Recent changes</p>
        <h1>Activity</h1>
        <p className="lede">
          Files received, starts, stops, address changes, and recovery appear
          here.
        </p>
      </div>
      <EventList
        bridge={bridge}
        events={events}
        recipes={recipes}
        instances={instances}
      />
    </section>
  );
}

function EventList({
  bridge,
  events,
  recipes,
  instances
}: {
  bridge: DesktopBridge;
  events: Event[];
  recipes: Recipe[];
  instances: Instance[];
}) {
  if (!events.length) {
    return (
      <div className="desktop-empty">
        <strong>No activity yet</strong>
        <p>Start a recipe to see what this computer does.</p>
      </div>
    );
  }
  return (
    <ol className="desktop-activity-list">
      {events.map((event) => {
        const receivedName =
          event.kind === "drop_file_received" &&
          typeof event.details?.itemName === "string" &&
          event.details.itemName
            ? event.details.itemName
            : "";
        return (
          <li key={event.id}>
            <StatusIcon
              status={
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
                  ? recipes.find(
                      (recipe) =>
                        recipe.id ===
                        instances.find(
                          (instance) => instance.id === event.instanceId
                        )?.recipeId
                    )?.title ?? "Spare"
                  : "Spare"}
              </strong>
              <p>{event.message}</p>
            </div>
            <time dateTime={event.createdAt}>{formatTime(event.createdAt)}</time>
            {receivedName && event.instanceId && (
              <button
                className="text-button activity-action"
                type="button"
                onClick={() =>
                  void bridge.RevealReceivedFile(
                    event.instanceId!,
                    receivedName
                  )
                }
              >
                Show file
              </button>
            )}
          </li>
        );
      })}
    </ol>
  );
}

function MachineView({ snapshot }: { snapshot: DesktopSnapshot }) {
  const machine = snapshot.machine;
  return (
    <section className="desktop-page">
      <div className="desktop-page-heading">
        <p className="eyebrow">This computer</p>
        <h1>{machine.hostname}</h1>
        <p className="lede">
          Spare uses this profile to choose safe defaults and explain what this
          computer can run.
        </p>
      </div>
      <dl className="machine-grid">
        <MachineMetric label="System" value={`${machine.os}/${machine.architecture}`} />
        <MachineMetric label="CPU" value={`${machine.logicalCores} logical cores`} />
        <MachineMetric label="Memory" value={formatBytes(machine.memoryTotalBytes)} />
        <MachineMetric
          label="Available storage"
          value={formatBytes(machine.storageAvailableBytes)}
        />
        <MachineMetric
          label="Local network"
          value={
            machine.lanAddresses.length
              ? machine.lanAddresses.join(", ")
              : "No address available"
          }
        />
      </dl>
      {machine.capabilities.hasBattery && (
        <p className="machine-warning">
          <span aria-hidden="true">△</span> This computer may sleep when its lid
          is closed.
        </p>
      )}
    </section>
  );
}

function SettingsView({
  bridge,
  snapshot,
  preferences,
  initialBackupPath,
  onSaved,
  onRepair,
  onConfigure,
  onRestored
}: {
  bridge: DesktopBridge;
  snapshot: DesktopSnapshot;
  preferences: DesktopPreferences;
  initialBackupPath: string;
  onSaved: () => Promise<void>;
  onRepair: () => void;
  onConfigure: () => void;
  onRestored: () => Promise<void>;
}) {
  const [values, setValues] = useState(preferences);
  const [working, setWorking] = useState(false);
  const [message, setMessage] = useState("");
  const [confirmUninstall, setConfirmUninstall] = useState(false);
  const [backupPath, setBackupPath] = useState(initialBackupPath);
  useEffect(() => {
    if (initialBackupPath) setBackupPath(initialBackupPath);
  }, [initialBackupPath]);
  async function save() {
    setWorking(true);
    setMessage("");
    try {
      await bridge.SavePreferences(values);
      await onSaved();
      setMessage("Settings saved.");
    } catch (requestError) {
      setMessage(errorMessage(requestError, "Unable to save settings."));
    } finally {
      setWorking(false);
    }
  }
  async function uninstall() {
    setWorking(true);
    setMessage("");
    try {
      await bridge.Uninstall();
    } catch (requestError) {
      setMessage(errorMessage(requestError, "Unable to uninstall Spare."));
      setWorking(false);
    }
  }
  async function chooseBackup() {
    try {
      const selected = await bridge.ChooseFile("backup");
      if (selected) {
        setBackupPath(selected);
        setMessage("Backup selected. Choose an empty destination to restore it.");
      }
    } catch (requestError) {
      setMessage(errorMessage(requestError, "Unable to open the backup picker."));
    }
  }
  async function exportBackup() {
    const instance = snapshot.instances[0];
    if (!instance) return;
    setWorking(true);
    setMessage("");
    try {
      const destination = await bridge.ExportBackup(instance.id);
      if (destination) setMessage(`Backup created at ${destination}.`);
    } catch (requestError) {
      setMessage(errorMessage(requestError, "Unable to export this backup."));
    } finally {
      setWorking(false);
    }
  }
  async function restoreBackup() {
    if (!backupPath) return;
    setWorking(true);
    setMessage("");
    try {
      await bridge.RestoreBackup(backupPath);
      setMessage("Backup restored.");
      await onRestored();
    } catch (requestError) {
      setMessage(errorMessage(requestError, "Unable to restore this backup."));
    } finally {
      setWorking(false);
    }
  }
  async function openDashboard() {
    setWorking(true);
    setMessage("");
    try {
      await bridge.OpenDashboard();
      setMessage("The remote dashboard opened in your browser.");
    } catch (requestError) {
      setMessage(
        errorMessage(requestError, "Unable to open the remote dashboard.")
      );
    } finally {
      setWorking(false);
    }
  }
  async function showStorage() {
    const path = snapshot.instances[0]?.dataPath;
    if (!path) return;
    setMessage("");
    try {
      await bridge.RevealPath(path);
    } catch (requestError) {
      setMessage(errorMessage(requestError, "Unable to show the selected folder."));
    }
  }
  return (
    <section className="desktop-page">
      <div className="desktop-page-heading">
        <p className="eyebrow">Local preferences</p>
        <h1>Settings</h1>
        <p className="lede">
          Control how the desktop app appears and when it notifies you.
        </p>
      </div>
      <div className="settings-groups">
        <section aria-labelledby="launch-heading">
          <h2 id="launch-heading">Launch behavior</h2>
          <label className="setting-row">
            <span>
              <strong>Show Spare in the menu bar</strong>
              <small>Keep fast controls available while the window is hidden.</small>
            </span>
            <input
              type="checkbox"
              checked={values.showInMenuBar}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  showInMenuBar: event.target.checked
                }))
              }
            />
          </label>
          <label className="setting-row">
            <span>
              <strong>Open Spare after login</strong>
              <small>The daemon can still restore jobs when this is off.</small>
            </span>
            <input
              type="checkbox"
              checked={values.openAfterLogin}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  openAfterLogin: event.target.checked
                }))
              }
            />
          </label>
          <label className="setting-row">
            <span>
              <strong>Keep installed recipes running after login</strong>
              <small>
                Restore the installed job when the background service starts.
              </small>
            </span>
            <input
              type="checkbox"
              checked={values.keepRecipesRunningAfterLogin}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  keepRecipesRunningAfterLogin: event.target.checked
                }))
              }
            />
          </label>
        </section>
        <section aria-labelledby="notifications-heading">
          <h2 id="notifications-heading">Notifications</h2>
          {["drop", "site", "hook"].map((recipeID) => (
            <label className="setting-row setting-row-nested" key={recipeID}>
              <span>
                <strong>
                  {recipeID === "drop"
                    ? "Drop notifications"
                    : recipeID === "site"
                      ? "Site notifications"
                      : "Hook notifications"}
                </strong>
                <small>
                  Choose whether this recipe can send operating-system alerts.
                </small>
              </span>
              <input
                type="checkbox"
                checked={values.recipeNotifications?.[recipeID] ?? true}
                disabled={!values.notifications}
                onChange={(event) =>
                  setValues((current) => ({
                    ...current,
                    recipeNotifications: {
                      ...current.recipeNotifications,
                      [recipeID]: event.target.checked
                    }
                  }))
                }
              />
            </label>
          ))}
          <label className="setting-row">
            <span>
              <strong>Send recipe notifications</strong>
              <small>Show file receipts and problems through the operating system.</small>
            </span>
            <input
              type="checkbox"
              checked={values.notifications}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  notifications: event.target.checked
                }))
              }
            />
          </label>
          <div className="settings-actions">
            <button
              className="button button-primary"
              type="button"
              onClick={() => void save()}
              disabled={working}
            >
              Save settings
            </button>
          </div>
        </section>
        <section aria-labelledby="diagnostics-heading">
          <h2 id="diagnostics-heading">Diagnostics</h2>
          <p>
            Repair uses the same initialization and service-registration path
            as the CLI.
          </p>
          <button
            className="button button-secondary"
            type="button"
            onClick={onRepair}
            disabled={working}
          >
            Run repair
          </button>
        </section>
        <section aria-labelledby="dashboard-access-heading">
          <h2 id="dashboard-access-heading">Dashboard access</h2>
          <p>
            Open the remote dashboard for this computer in your browser. Spare
            creates a new single-use session without exposing its permanent
            local credential.
          </p>
          <button
            className="button button-secondary"
            type="button"
            onClick={() => void openDashboard()}
            disabled={working}
          >
            Open remote dashboard
          </button>
        </section>
        <section aria-labelledby="recipe-storage-heading">
          <h2 id="recipe-storage-heading">Recipe storage</h2>
          {snapshot.instances[0]?.dataPath ? (
            <>
              <p>
                {titleForRecipe(
                  snapshot.recipes,
                  snapshot.instances[0].recipeId
                )}{" "}
                uses this selected folder. Spare will not delete its contents.
              </p>
              <code className="settings-path">
                {snapshot.instances[0].dataPath}
              </code>
              <div className="actions">
                <button
                  className="button button-secondary"
                  type="button"
                  onClick={() => void showStorage()}
                  disabled={working}
                >
                  Show selected folder
                </button>
                <button
                  className="button button-secondary"
                  type="button"
                  onClick={onConfigure}
                  disabled={working}
                >
                  Choose another folder
                </button>
              </div>
            </>
          ) : (
            <p>
              The active recipe does not use a selected folder. Choose another
              job from Recipes if you want Spare to work with local files.
            </p>
          )}
        </section>
        <section aria-labelledby="backup-heading">
          <h2 id="backup-heading">Backup and restore</h2>
          <p>
            Backups include the active recipe configuration and a copy of its
            selected folder. Original files are never moved.
          </p>
          <div className="actions">
            {snapshot.instances[0]?.dataPath && (
              <button
                className="button button-secondary"
                type="button"
                onClick={() => void exportBackup()}
                disabled={working}
              >
                Export backup
              </button>
            )}
            <button
              className="button button-secondary"
              type="button"
              onClick={() => void chooseBackup()}
              disabled={working}
            >
              Choose backup
            </button>
          </div>
          {backupPath && (
            <div className="backup-selection">
              <strong>Selected backup</strong>
              <code>{backupPath}</code>
              <button
                className="button button-primary"
                type="button"
                onClick={() => void restoreBackup()}
                disabled={working || Boolean(snapshot.instances[0])}
              >
                Choose destination and restore
              </button>
              {snapshot.instances[0] && (
                <p className="remove-note">
                  Remove the current job before restoring another one.
                </p>
              )}
            </div>
          )}
        </section>
        <section className="danger-zone" aria-labelledby="uninstall-heading">
          <h2 id="uninstall-heading">Uninstall</h2>
          <p>
            Uninstalling stops Spare and removes its local state. Selected
            folders and received files remain unchanged.
          </p>
          {!confirmUninstall ? (
            <button
              className="button button-danger"
              type="button"
              onClick={() => setConfirmUninstall(true)}
            >
              Uninstall Spare
            </button>
          ) : (
            <div className="actions">
              <button
                className="button button-danger"
                type="button"
                onClick={() => void uninstall()}
                disabled={working}
              >
                Uninstall Spare now
              </button>
              <button
                className="button button-secondary"
                type="button"
                onClick={() => setConfirmUninstall(false)}
                disabled={working}
              >
                Cancel
              </button>
            </div>
          )}
        </section>
      </div>
      <div className="sr-only" role="status">
        {message}
      </div>
      {message && <p className="settings-message">{message}</p>}
    </section>
  );
}

function MachineMetric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function NavIcon({ name }: { name: View }) {
  const symbols: Record<View, string> = {
    home: "⌂",
    recipes: "◇",
    activity: "↻",
    machine: "▣",
    settings: "⚙"
  };
  return <span aria-hidden="true">{symbols[name]}</span>;
}

function RecipeIcon({ recipe }: { recipe: string }) {
  const symbols: Record<string, string> = {
    drop: "↓",
    site: "□",
    hook: "⌁"
  };
  return (
    <span className={`recipe-icon recipe-icon-${recipe}`} aria-hidden="true">
      {symbols[recipe] ?? "◇"}
    </span>
  );
}

function StatusIcon({ status }: { status?: Instance["status"] }) {
  const tone = status ?? "stopped";
  return (
    <span
      className={`status-dot status-dot-${tone}`}
      aria-hidden="true"
    />
  );
}

function titleForRecipe(recipes: Recipe[], id: string) {
  return recipes.find((recipe) => recipe.id === id)?.title ?? id;
}

function shortDescription(id: string) {
  if (id === "drop") return "Receive and send files nearby";
  if (id === "site") return "Serve a folder locally";
  if (id === "hook") return "Capture and replay webhooks";
  return "Give this computer a useful job";
}

function readyDescription(id: string) {
  if (id === "drop") return "Drop is ready to receive files from nearby devices.";
  if (id === "site") return "Site is serving its selected folder read-only.";
  if (id === "hook") return "Hook is ready to receive and inspect requests.";
  return "This job is ready.";
}

function formatBytes(value: number) {
  if (!value) return "Unavailable";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unit = 0;
  while (size >= 1000 && unit < units.length - 1) {
    size /= 1000;
    unit += 1;
  }
  return `${size >= 10 || unit === 0 ? size.toFixed(0) : size.toFixed(1)} ${units[unit]}`;
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(new Date(value));
}

function errorMessage(value: unknown, fallback: string) {
  return value instanceof Error && value.message ? value.message : fallback;
}
