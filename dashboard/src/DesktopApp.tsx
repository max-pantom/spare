import {
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState
} from "react";
import type { RefObject } from "react";
import QRCode from "qrcode";
import type { DesktopBridge } from "./desktop";
import { presentEvent, presentProblem } from "./presentation";
import type {
  DesktopPreferences,
  DesktopSnapshot,
  DesktopTheme,
  Event,
  Instance,
  Recipe
} from "./types";
import { SpareNavIcon, type SpareNavIconName } from "./SpareNavIcon";

type View = "home" | "recipes" | "activity" | "machine" | "settings";

const navigation: Array<{
  id: View;
  label: string;
  icon: SpareNavIconName;
}> = [
  { id: "home", label: "Home", icon: "home" },
  { id: "recipes", label: "Jobs", icon: "jobs" },
  { id: "activity", label: "Activity", icon: "activity" },
  { id: "machine", label: "Computer", icon: "computer" },
  { id: "settings", label: "Settings", icon: "settings" }
];

const transformationStepDelay = 900;
const transformationCompleteDelay = 1800;
const transformationStepCount = 4;

type TransformationState = {
  step: number;
  complete: boolean;
};

type TransformationStage = "" | "starting" | "complete";

const statusLabels: Record<Instance["status"], string> = {
  starting: "Starting",
  healthy: "Ready",
  degraded: "Needs attention",
  stopped: "Stopped",
  failed: "Needs attention",
  removing: "Removing"
};

const desktopThemes: Array<{
  id: DesktopTheme;
  label: string;
}> = [
  {
    id: "dark",
    label: "Dark"
  },
  {
    id: "light",
    label: "Light"
  }
];

export function DesktopApp({ bridge }: { bridge: DesktopBridge }) {
  const [snapshot, setSnapshot] = useState<DesktopSnapshot>();
  const [view, setView] = useState<View>("home");
  const [detailRecipe, setDetailRecipe] = useState<Recipe>();
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
  const [themePreview, setThemePreview] = useState<DesktopTheme>();
  const [transformationStage, setTransformationStage] =
    useState<TransformationStage>("");

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
          setDetailRecipe(undefined);
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
          destination === "settings" ||
          destination === "share"
        ) {
          if (destination === "share") {
            setView("home");
            setDetailRecipe(undefined);
            setShowShare(true);
          } else {
            setDetailRecipe(undefined);
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
  const windowTitle = transformationStage === "starting" && setupRecipe
    ? `Starting ${setupRecipe.title}`
    : transformationStage === "complete" && setupRecipe
      ? `This computer is now a ${setupRecipe.title}`
      : instance
        ? `This computer is a ${activeRecipe?.title ?? instance.recipeId}`
        : setupRecipe
          ? `Set up ${setupRecipe.title}`
          : detailRecipe
            ? detailRecipe.title
            : view === "home"
              ? "Spare"
              : navigation.find((item) => item.id === view)?.label ?? "Spare";
  const windowState = windowStateFor(
    instance,
    loading,
    error,
    transformationStage
  );

  useEffect(() => {
    document.title = `${windowTitle} · Spare`;
  }, [windowTitle]);

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
    if (transformationStage) return;
    setView(destination);
    setDetailRecipe(undefined);
    setSetupRecipe(undefined);
    setEditingInstance(undefined);
    setSetupInitialValues({});
    setShowShare(false);
    setError("");
    if (destination === "machine") void refresh();
    window.requestAnimationFrame(() => {
      const main = document.getElementById("desktop-main");
      main?.scrollTo({ top: 0, left: 0 });
      main?.focus({ preventScroll: true });
    });
  }

  return (
    <div
      className="desktop-root"
      data-theme={themePreview ?? snapshot?.preferences.theme ?? "dark"}
    >
      <a className="skip-link" href="#desktop-main">
        Skip to content
      </a>
      <header className="desktop-titlebar">
        {!window.runtime && (
          <span className="window-controls" aria-hidden="true">
            <span className="window-control window-control-close" />
            <span className="window-control window-control-minimize" />
            <span className="window-control window-control-zoom" />
          </span>
        )}
        <p>{windowTitle}</p>
        <span className={`desktop-window-state state-${windowState.tone}`}>
          <span aria-hidden="true" />
          {windowState.label}
        </span>
      </header>
      <aside className="desktop-sidebar">
        <nav className="desktop-nav" aria-label="Primary">
          {navigation.map((item) => (
            <button
              key={item.id}
              type="button"
              aria-current={view === item.id ? "page" : undefined}
              disabled={Boolean(transformationStage)}
              onClick={() => navigate(item.id)}
            >
              <SpareNavIcon
                className="nav-icon"
                name={item.icon}
                size={14}
              />
              {item.label}
            </button>
          ))}
        </nav>
      </aside>

      <main
        id="desktop-main"
        className="desktop-main"
        aria-busy={loading}
        tabIndex={-1}
      >
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
            onTransformationStageChange={setTransformationStage}
            backLabel={detailRecipe ? `Back to ${detailRecipe.title}` : "Back to jobs"}
            onBack={() => {
              setSetupRecipe(undefined);
              setEditingInstance(undefined);
            }}
            onStarted={async (created) => {
              const wasEditing = Boolean(editingInstance);
              setSnapshot((current) =>
                current ? { ...current, instances: [created] } : current
              );
              setTransformationStage("");
              setDetailRecipe(undefined);
              setSetupRecipe(undefined);
              setEditingInstance(undefined);
              setView("home");
              setAnnouncement(
                wasEditing
                  ? `${setupRecipe.title} configuration was updated.`
                  : `This computer is now a ${setupRecipe.title}.`
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
                  setDetailRecipe(recipe);
                  setView("recipes");
                }}
                onStart={() => void lifecycle("start")}
                onStop={() => void lifecycle("stop")}
                onAddFiles={() => void addFilesToDrop()}
                onActivity={() => navigate("activity")}
                onRepair={() => void repair()}
                onConfigure={() => {
                  if (instance && activeRecipe) {
                    setDetailRecipe(undefined);
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
              detailRecipe ? (
                <JobDetailView
                  recipe={detailRecipe}
                  snapshot={snapshot}
                  onBack={() => setDetailRecipe(undefined)}
                  onSetup={() => {
                    setSetupInitialValues({});
                    setEditingInstance(undefined);
                    setSetupRecipe(detailRecipe);
                  }}
                />
              ) : (
                <RecipesView
                  bridge={bridge}
                  snapshot={snapshot}
                  onChoose={(recipe) => {
                    setSetupInitialValues({});
                    setEditingInstance(undefined);
                    setDetailRecipe(recipe);
                  }}
                />
              )
            )}
            {view === "activity" && (
              <ActivityView
                bridge={bridge}
                snapshot={snapshot}
                working={working}
                onRepair={() => void repair()}
                onDetails={() => navigate("machine")}
                onStop={() => void lifecycle("stop")}
                onConfigure={() => {
                  if (instance && activeRecipe) {
                    setDetailRecipe(undefined);
                    setSetupInitialValues(instance.config);
                    setEditingInstance(instance);
                    setSetupRecipe(activeRecipe);
                  }
                }}
              />
            )}
            {view === "machine" && <MachineView snapshot={snapshot} />}
            {view === "settings" && (
              <SettingsView
                bridge={bridge}
                snapshot={snapshot}
                preferences={snapshot.preferences}
                initialBackupPath={droppedBackup}
                onThemeChange={setThemePreview}
                onSaved={refresh}
                onRepair={() => void repair()}
                onConfigure={() => {
                  if (instance && activeRecipe) {
                    setDetailRecipe(undefined);
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
  onRepair,
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
  onRepair: () => void;
  onConfigure: () => void;
  onRecipes: () => void;
  onMachine: () => void;
}) {
  const scanButtonRef = useRef<HTMLButtonElement>(null);
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
  const latestEvent = snapshot.events.find(
    (event) => event.instanceId === instance.id
  );
  const metrics = overviewMetrics(instance, latestEvent);
  const primaryURL = instance.urls[0];
  return (
    <section className="desktop-page desktop-overview">
      <h1 className="sr-only">This computer is a {title}</h1>
      {instance.problem ? (
        <DesktopFailureState
          instance={instance}
          title={title}
          working={working}
          onRepair={onRepair}
          onDetails={onActivity}
          onReconnect={onConfigure}
          onStop={onStop}
        />
      ) : (
        <p className={`desktop-ready status-${instance.status}`}>
          {instance.status === "healthy"
            ? compactReadyDescription(instance.recipeId)
            : `${title} is ${statusLabels[instance.status].toLowerCase()}.`}
        </p>
      )}

      {!instance.problem && (
        <div
          className="desktop-toolbar"
          role="group"
          aria-label={`${title} controls`}
        >
        <button
          ref={scanButtonRef}
          className="button button-primary"
          type="button"
          onClick={onShowShare}
          aria-expanded={showShare}
          disabled={!primaryURL}
        >
          Scan QR
        </button>
        <button
          className="button button-secondary"
          type="button"
          onClick={() => primaryURL && void bridge.OpenURL(primaryURL)}
          disabled={!primaryURL}
        >
          Open {title}
        </button>
        {instance.recipeId === "drop" ? (
          <button
            className="button button-secondary"
            type="button"
            onClick={() =>
              instance.dataPath && void bridge.RevealPath(instance.dataPath)
            }
            disabled={!instance.dataPath}
          >
            Open Received files
          </button>
        ) : (
          <button
            className="button button-secondary"
            type="button"
            onClick={onConfigure}
          >
            Configure
          </button>
        )}
        {running ? (
          <button
            className="button button-secondary"
            type="button"
            onClick={onStop}
            disabled={working}
          >
            Pause
          </button>
        ) : (
          <button
            className="button button-primary"
            type="button"
            onClick={onStart}
            disabled={working}
          >
            Resume
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
        {instance.recipeId === "drop" && (
          <button
            className="button button-secondary"
            type="button"
            onClick={onConfigure}
          >
            Configure
          </button>
        )}
        <button
          className="button button-secondary"
          type="button"
          onClick={onActivity}
        >
          View activity
        </button>
        </div>
      )}

      <h2 className="desktop-info-heading">Info</h2>
      <dl className="desktop-metrics" aria-label={`${title} status`}>
        {metrics.map((metric) => (
          <div key={metric.label}>
            <dt>{metric.label}</dt>
            <dd>{metric.value}</dd>
          </div>
        ))}
      </dl>

      {showShare && (
        <SharePanel
          instance={instance}
          bridge={bridge}
          title={title}
          onClose={onShowShare}
          returnFocusRef={scanButtonRef}
        />
      )}

      {instance.recipeId === "drop" && <RecipeSignal />}
    </section>
  );
}

function SharePanel({
  instance,
  bridge,
  title,
  onClose,
  returnFocusRef
}: {
  instance: Instance;
  bridge: DesktopBridge;
  title: string;
  onClose: () => void;
  returnFocusRef: RefObject<HTMLButtonElement | null>;
}) {
  const shareURL =
    instance.urls.find(
      (value) => !value.includes("127.0.0.1") && !value.includes(".local")
    ) ??
    instance.urls[0] ??
    "";
  const [qrCode, setQrCode] = useState("");
  const dialogRef = useRef<HTMLDialogElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (!dialog.open) dialog.showModal();
    closeButtonRef.current?.focus();
    return () => {
      if (dialog.open) dialog.close();
      window.requestAnimationFrame(() => returnFocusRef.current?.focus());
    };
  }, [returnFocusRef]);

  useEffect(() => {
    if (!shareURL) return;
    void QRCode.toDataURL(shareURL, {
      width: 256,
      margin: 2,
      color: { dark: "#151515", light: "#ffffff" }
    }).then(setQrCode);
  }, [shareURL]);
  return (
    <dialog
      ref={dialogRef}
      className="desktop-share-dialog"
      aria-labelledby="share-heading"
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onMouseDown={(event) => {
        const bounds = event.currentTarget.getBoundingClientRect();
        const outside =
          event.clientX < bounds.left ||
          event.clientX > bounds.right ||
          event.clientY < bounds.top ||
          event.clientY > bounds.bottom;
        if (outside) onClose();
      }}
    >
      <section className="desktop-share">
        <button
          ref={closeButtonRef}
          className="desktop-share-close"
          type="button"
          aria-label="Close QR code"
          onClick={onClose}
        >
          <span aria-hidden="true">×</span>
        </button>
        <div className="desktop-share-copy">
          <p className="eyebrow">Nearby access</p>
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
          {qrCode ? (
            <img
              src={qrCode}
              alt={`QR code for ${shareURL}`}
              width="176"
              height="176"
            />
          ) : (
            <span className="desktop-qr-placeholder" role="status">
              Preparing code…
            </span>
          )}
          <code>{shareURL}</code>
        </div>
      </section>
    </dialog>
  );
}

function SetupView({
  bridge,
  recipe,
  initialValues,
  existingInstance,
  onTransformationStageChange,
  backLabel,
  onBack,
  onStarted
}: {
  bridge: DesktopBridge;
  recipe: Recipe;
  initialValues: Record<string, unknown>;
  existingInstance?: Instance;
  onTransformationStageChange: (stage: TransformationStage) => void;
  backLabel: string;
  onBack: () => void;
  onStarted: (instance: Instance) => void | Promise<void>;
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
  const [transformation, setTransformation] =
    useState<TransformationState>();

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
    const transformationController = new AbortController();
    try {
      const input = {
        recipeId: recipe.id,
        mode,
        config: values,
        port: existingInstance?.portMode === "fixed" ? existingInstance.port : 0,
        portMode: existingInstance?.portMode ?? ("auto" as const)
      };
      if (existingInstance) {
        const configured = await bridge.ConfigureInstance(
          existingInstance.id,
          input
        );
        await onStarted(configured);
        return;
      }

      setTransformation({ step: 0, complete: false });
      onTransformationStageChange("starting");
      const [created] = await Promise.all([
        bridge.CreateInstance(input),
        playTransformationSequence(
          (step) => setTransformation({ step, complete: false }),
          transformationController.signal
        )
      ]);
      setTransformation({
        step: transformationStepCount - 1,
        complete: true
      });
      onTransformationStageChange("complete");
      await waitForTransformation(
        transformationCompleteDelay,
        transformationController.signal
      );
      await onStarted(created);
    } catch (requestError) {
      transformationController.abort();
      setTransformation(undefined);
      setError(
        errorMessage(
          requestError,
          `Unable to start ${recipe.title}. Review its settings and try again.`
        )
      );
    } finally {
      transformationController.abort();
      onTransformationStageChange("");
      setWorking(false);
    }
  }

  const transformationAnnouncement = transformation
    ? transformation.complete
      ? `This computer is now a ${recipe.title}.`
      : transformationSteps(recipe.title)[transformation.step]
    : "";

  return (
    <>
      <div
        className="sr-only"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >
        {transformationAnnouncement}
      </div>
      {transformation ? (
        <TransformationView
          title={recipe.title}
          step={transformation.step}
          complete={transformation.complete}
        />
      ) : (
      <section className="desktop-page setup-page">
      <button className="back-button" type="button" onClick={onBack}>
        <span aria-hidden="true">←</span> {backLabel}
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
      )}
    </>
  );
}

function TransformationView({
  title,
  step,
  complete
}: {
  title: string;
  step: number;
  complete: boolean;
}) {
  const steps = transformationSteps(title);
  return (
    <section
      className={`desktop-page transformation-page${complete ? " is-complete" : ""}`}
      aria-labelledby="transformation-heading"
    >
      <div className="transformation-content">
        <p className="eyebrow">{complete ? "Job ready" : "Starting a job"}</p>
        <h1 id="transformation-heading">
          {complete
            ? `This computer is now a ${title}.`
            : `Preparing ${title}`}
        </h1>
        <ol
          className="transformation-steps"
          data-progress={complete ? "complete" : step}
          aria-label={`${title} startup progress`}
        >
          {steps.map((label, index) => {
            const state = complete || index < step
              ? "done"
              : index === step
                ? "current"
                : "pending";
            return (
              <li
                className={`transformation-step is-${state}`}
                aria-current={state === "current" ? "step" : undefined}
                key={label}
              >
                <span className="transformation-marker" aria-hidden="true">
                  {state === "done" ? "✓" : ""}
                </span>
                <span>{label}</span>
                <span className="sr-only">
                  {state === "done"
                    ? " — complete"
                    : state === "current"
                      ? " — in progress"
                      : " — waiting"}
                </span>
              </li>
            );
          })}
        </ol>
      </div>
    </section>
  );
}

type JobDetailCopy = {
  headline: string;
  summary: string;
  benefits: string[];
  permissions: string[];
};

const jobDetailCopy: Record<string, JobDetailCopy> = {
  drop: {
    headline: "Turn this computer into a nearby file receiver.",
    summary:
      "Send files from phones, tablets, and other computers without uploading them to the cloud.",
    benefits: [
      "Receives files over the local network",
      "Stores them in a folder you choose",
      "Shows recent transfers",
      "Works from any nearby browser"
    ],
    permissions: [
      "Receive local network connections",
      "Write into Downloads/Spare",
      "Run after login"
    ]
  },
  site: {
    headline: "Turn this computer into a private local website.",
    summary:
      "Share a folder with nearby devices in a browser while its files stay on this computer.",
    benefits: [
      "Serves a folder as a read-only website",
      "Works over the local network",
      "Opens from any nearby browser",
      "Keeps the original files on this computer"
    ],
    permissions: [
      "Receive local network connections",
      "Read the folder you choose",
      "Run after login"
    ]
  },
  hook: {
    headline: "Turn this computer into a local webhook receiver.",
    summary:
      "Inspect requests from nearby apps and devices without sending their data to a cloud service.",
    benefits: [
      "Receives local webhook requests",
      "Shows headers and request bodies",
      "Keeps a recent request history",
      "Works from nearby development devices"
    ],
    permissions: [
      "Receive local network connections",
      "Store recent requests locally",
      "Run after login"
    ]
  }
};

function JobDetailView({
  recipe,
  snapshot,
  onBack,
  onSetup
}: {
  recipe: Recipe;
  snapshot: DesktopSnapshot;
  onBack: () => void;
  onSetup: () => void;
}) {
  const copy = jobDetailCopy[recipe.id] ?? {
    headline: recipe.description,
    summary:
      "Run this trusted job locally and keep its working data on this computer.",
    benefits: [
      "Runs on this computer",
      "Uses the local network when needed",
      "Keeps working data nearby",
      "Can run again after login"
    ],
    permissions: recipe.permissions.map(
      (permission) => permission.description
    )
  };
  const machine = snapshot.machine;
  const networkAvailable =
    machine.capabilities.canServeLAN || machine.lanAddresses.length > 0;
  const hasCurrentJob = snapshot.instances.length > 0;
  const canSetUp = recipe.compatibility.supported && !hasCurrentJob;

  return (
    <article className="desktop-page job-detail-page">
      <button className="back-button" type="button" onClick={onBack}>
        <span aria-hidden="true">←</span> Back to jobs
      </button>

      <header className="job-detail-header">
        <p className="eyebrow">{recipe.title}</p>
        <h1>{copy.headline}</h1>
        <p className="lede">{copy.summary}</p>
      </header>

      <section className="job-detail-section" aria-labelledby="job-does-heading">
        <h2 id="job-does-heading">What it does</h2>
        <ul className="job-detail-list">
          {copy.benefits.map((benefit) => (
            <li key={benefit}>
              <span className="job-detail-marker" aria-hidden="true">✓</span>
              <span>{benefit}</span>
            </li>
          ))}
        </ul>
      </section>

      <section
        className="job-detail-section"
        aria-labelledby="job-computer-heading"
      >
        <h2 id="job-computer-heading">This computer</h2>
        <ul className="job-machine-list">
          <li>
            <span className="job-detail-marker" aria-hidden="true">✓</span>
            <strong>{recipe.compatibility.rating} for {recipe.title}</strong>
          </li>
          <li>
            <span className="job-detail-marker" aria-hidden="true">✓</span>
            <strong>{formatBytes(machine.storageAvailableBytes)} available</strong>
          </li>
          <li>
            <span className="job-detail-marker" aria-hidden="true">
              {networkAvailable ? "✓" : "—"}
            </span>
            <strong>
              {networkAvailable
                ? "Local network available"
                : "Local network unavailable"}
            </strong>
          </li>
          <li>
            <span className="job-detail-marker" aria-hidden="true">✓</span>
            <strong>
              {machine.capabilities.hasBattery
                ? "Battery powered"
                : "No battery limits"}
            </strong>
          </li>
        </ul>
      </section>

      <section
        className="job-detail-section job-permission-section"
        aria-labelledby="job-permissions-heading"
      >
        <h2 id="job-permissions-heading">
          {recipe.title} needs access to
        </h2>
        <ul className="job-detail-list">
          {copy.permissions.map((permission) => (
            <li key={permission}>
              <span className="job-detail-marker" aria-hidden="true">✓</span>
              <span>{permission}</span>
            </li>
          ))}
        </ul>
      </section>

      <div className="job-detail-action">
        <button
          className="button button-primary"
          type="button"
          onClick={onSetup}
          disabled={!canSetUp}
        >
          Set up {recipe.title}
        </button>
        {!recipe.compatibility.supported && (
          <p>This computer does not currently support {recipe.title}.</p>
        )}
        {hasCurrentJob && (
          <p>Change the current job before setting up another one.</p>
        )}
      </div>
    </article>
  );
}

function RecipesView({
  bridge,
  snapshot,
  onChoose
}: {
  bridge: DesktopBridge;
  snapshot: DesktopSnapshot;
  onChoose: (recipe: Recipe) => void;
}) {
  const [error, setError] = useState("");
  const installedJobs = [...snapshot.recipes].sort(
    (first, second) => installedJobRank(first.id) - installedJobRank(second.id)
  );

  async function installMoreJobs() {
    try {
      await bridge.OpenRecipePackage("");
      setError("");
    } catch (requestError) {
      setError(
        errorMessage(
          requestError,
          "Unable to open a job package. No installed jobs were changed."
        )
      );
    }
  }

  return (
    <section className="desktop-page desktop-jobs-page">
      <div className="desktop-jobs-heading">
        <h1>Installed Jobs</h1>
        <p>
          Choose one useful job for this computer. Give this computer something
          new to do.
        </p>
      </div>
      {error && (
        <div className="form-error" role="alert">
          {error}
        </div>
      )}
      <div className="desktop-job-cards" aria-label="Installed jobs">
        {installedJobs.map((recipe) => (
          <article className="desktop-job-card" key={recipe.id}>
            <div className="desktop-job-card-heading">
              <h2>{recipe.title}</h2>
              <button
                className="desktop-job-card-action"
                type="button"
                onClick={() => onChoose(recipe)}
                disabled={!recipe.compatibility.supported}
              >
                {recipe.id === "hook" ? "Open" : "Start"}
              </button>
            </div>
            <p>{installedJobDescription(recipe.id)}</p>
            <footer>
              <span
                className={
                  recipe.compatibility.supported
                    ? "desktop-job-state"
                    : "desktop-job-state is-unavailable"
                }
              >
                {recipe.compatibility.supported ? "Active" : "Unavailable"}
              </span>
              <span>Pre-installed</span>
            </footer>
          </article>
        ))}
      </div>
      <button
        className="desktop-jobs-install"
        type="button"
        onClick={() => void installMoreJobs()}
      >
        Install more jobs
      </button>
    </section>
  );
}

const installedJobOrder = ["drop", "hook", "site"];

function installedJobRank(id: string) {
  const rank = installedJobOrder.indexOf(id);
  return rank === -1 ? installedJobOrder.length : rank;
}

function installedJobDescription(id: string) {
  if (id === "drop") {
    return "Receive files directly from nearby phones, tablets, and computers over your local network.";
  }
  if (id === "hook") {
    return "Capture, inspect, and replay webhook requests while testing apps, integrations, and automations.";
  }
  if (id === "site") {
    return "Turn any folder into a local website that nearby devices can open through their browser.";
  }
  return "Give this computer something useful to do.";
}

function ActivityView({
  bridge,
  snapshot,
  working,
  onRepair,
  onDetails,
  onStop,
  onConfigure
}: {
  bridge: DesktopBridge;
  snapshot: DesktopSnapshot;
  working: boolean;
  onRepair: () => void;
  onDetails: () => void;
  onStop: () => void;
  onConfigure: () => void;
}) {
  const instance = snapshot.instances[0];
  const recipe = snapshot.recipes.find(
    (available) => available.id === instance?.recipeId
  );
  const title = recipe?.title ?? instance?.recipeId ?? "Job";

  return (
    <section className="desktop-page activity-page">
      <div className="desktop-page-heading">
        <p className="eyebrow">System history</p>
        <h1>Activity</h1>
        <p className="lede">A human-readable history of what this computer has done.</p>
      </div>
      {instance?.problem && (
        <DesktopFailureState
          instance={instance}
          title={title}
          working={working}
          onRepair={onRepair}
          onDetails={onDetails}
          onReconnect={onConfigure}
          onStop={onStop}
        />
      )}
      <EventList
        bridge={bridge}
        events={snapshot.events}
        recipes={snapshot.recipes}
        instances={snapshot.instances}
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
        const instance = instances.find(
          (available) => available.id === event.instanceId
        );
        const title =
          recipes.find(
            (recipe) =>
              recipe.id === (instance?.recipeId ?? event.instanceId)
          )?.title ?? "Spare";
        const presented = presentEvent(event, title);
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
              <strong>{presented.summary}</strong>
              {presented.detail && <p>{presented.detail}</p>}
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

function DesktopFailureState({
  instance,
  title,
  working,
  onRepair,
  onDetails,
  onReconnect,
  onStop
}: {
  instance: Instance;
  title: string;
  working: boolean;
  onRepair: () => void;
  onDetails: () => void;
  onReconnect: () => void;
  onStop: () => void;
}) {
  const failure = presentProblem(instance, title);
  return (
    <section
      className="desktop-failure-state"
      aria-labelledby="desktop-failure-heading"
    >
      <p className="eyebrow status-failed">
        <StatusIcon status="failed" /> Needs attention
      </p>
      <h2 id="desktop-failure-heading">{failure.title}</h2>
      <p>{failure.explanation}</p>
      <div className="actions">
        {failure.storageProblem ? (
          <>
            <button
              className="button button-primary"
              type="button"
              onClick={onRepair}
              disabled={working}
            >
              Reconnect folder
            </button>
            <button
              className="button button-secondary"
              type="button"
              onClick={onReconnect}
            >
              Choose another folder
            </button>
          </>
        ) : (
          <>
            <button
              className="button button-primary"
              type="button"
              onClick={onRepair}
              disabled={working}
            >
              Run repair
            </button>
            <button
              className="button button-secondary"
              type="button"
              onClick={onDetails}
            >
              View details
            </button>
          </>
        )}
        <button
          className="button button-secondary"
          type="button"
          onClick={onStop}
          disabled={working}
        >
          {failure.storageProblem ? "Stop job" : `Stop ${title}`}
        </button>
      </div>
    </section>
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
      <section
        className="machine-technical-section"
        aria-labelledby="desktop-machine-technical-heading"
      >
        <h2 id="desktop-machine-technical-heading">Technical details</h2>
        <dl className="machine-grid">
          <MachineMetric label="System" value={machine.os} />
          <MachineMetric label="Architecture" value={machine.architecture} />
          <MachineMetric
            label="CPU"
            value={`${machine.logicalCores} logical cores`}
          />
          <MachineMetric
            label="Memory"
            value={formatBytes(machine.memoryTotalBytes)}
          />
          <MachineMetric
            label="Storage"
            value={formatBytes(machine.storageAvailableBytes)}
          />
          <MachineMetric
            label="Network"
            value={
              machine.lanAddresses.length
                ? machine.lanAddresses.join(", ")
                : "No address available"
            }
            technical
          />
          <MachineMetric
            label="Battery"
            value={machine.capabilities.hasBattery ? "Battery powered" : "No battery"}
          />
          <MachineMetric
            label="External drives"
            value={
              machine.capabilities.hasExternalStorage
                ? "Detected"
                : "None detected"
            }
          />
          <MachineMetric
            label="Container support"
            value={
              machine.capabilities.canRunContainers
                ? "Available"
                : "Unavailable"
            }
          />
        </dl>
      </section>
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
  onThemeChange,
  onSaved,
  onRepair,
  onConfigure,
  onRestored
}: {
  bridge: DesktopBridge;
  snapshot: DesktopSnapshot;
  preferences: DesktopPreferences;
  initialBackupPath: string;
  onThemeChange: (theme: DesktopTheme) => void;
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
        </section>
        <section aria-labelledby="appearance-heading">
          <h2 id="appearance-heading">Appearance</h2>
          <p>Choose how the Spare window looks on this computer.</p>
          <fieldset className="theme-picker">
            <legend className="sr-only">Theme</legend>
            {desktopThemes.map((theme) => (
              <label className="theme-choice" key={theme.id}>
                <span
                  className={`theme-preview theme-preview-${theme.id}`}
                  aria-hidden="true"
                >
                  <span />
                </span>
                <span>
                  <strong>{theme.label}</strong>
                </span>
                <input
                  type="radio"
                  name="desktop-theme"
                  value={theme.id}
                  checked={values.theme === theme.id}
                  onChange={() => {
                    setValues((current) => ({
                      ...current,
                      theme: theme.id
                    }));
                    onThemeChange(theme.id);
                  }}
                />
              </label>
            ))}
          </fieldset>
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

function MachineMetric({
  label,
  value,
  technical = false
}: {
  label: string;
  value: string;
  technical?: boolean;
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd className={technical ? "technical-value" : undefined}>{value}</dd>
    </div>
  );
}

function RecipeSignal() {
  return (
    <div className="recipe-signal recipe-signal-drop" aria-hidden="true">
      <span className="recipe-signal-mark" />
    </div>
  );
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

function compactReadyDescription(id: string) {
  if (id === "drop") return "Ready to receive files";
  if (id === "site") return "Serving one folder read-only";
  if (id === "hook") return "Ready to capture requests";
  return "Ready";
}

function overviewMetrics(instance: Instance, latestEvent?: Event) {
  const lastActivity = latestEvent
    ? `${latestEvent.message} ${formatRelativeTime(latestEvent.createdAt)}`
    : "No activity yet";
  if (instance.recipeId === "drop") {
    return [
      { label: "Files received today", value: String(instance.itemCount) },
      {
        label: "Storage available",
        value: formatBytes(instance.storageAvailableBytes)
      },
      { label: "Last activity", value: lastActivity }
    ];
  }
  if (instance.recipeId === "hook") {
    return [
      { label: "Requests captured", value: String(instance.itemCount) },
      { label: "Local endpoint", value: compactAddress(instance.urls[0]) },
      { label: "Last activity", value: lastActivity }
    ];
  }
  return [
    { label: "Folder", value: finalPathPart(instance.dataPath) },
    { label: "Access", value: "Read-only" },
    { label: "Local address", value: compactAddress(instance.urls[0]) }
  ];
}

function finalPathPart(path: string) {
  if (!path) return "No folder selected";
  const parts = path.replaceAll("\\", "/").split("/").filter(Boolean);
  return parts.at(-1) ?? path;
}

function compactAddress(url?: string) {
  if (!url) return "Unavailable";
  try {
    const parsed = new URL(url);
    return parsed.host;
  } catch {
    return url;
  }
}

function formatRelativeTime(value: string) {
  const elapsed = Date.now() - new Date(value).getTime();
  if (!Number.isFinite(elapsed) || elapsed < 0) return "just now";
  const minutes = Math.floor(elapsed / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes} ${minutes === 1 ? "minute" : "minutes"} ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} ${hours === 1 ? "hour" : "hours"} ago`;
  const days = Math.floor(hours / 24);
  return `${days} ${days === 1 ? "day" : "days"} ago`;
}

function windowStateFor(
  instance: Instance | undefined,
  loading: boolean,
  error: string,
  transformationStage: TransformationStage = ""
) {
  if (error || instance?.status === "failed") {
    return { label: "Failed", tone: "failed" };
  }
  if (instance?.status === "degraded") {
    return { label: "Attention", tone: "warning" };
  }
  if (transformationStage === "complete") {
    return { label: "Working", tone: "working" };
  }
  if (
    transformationStage === "starting" ||
    loading ||
    instance?.status === "starting"
  ) {
    return { label: "Starting", tone: "working" };
  }
  if (instance?.status === "stopped") {
    return { label: "Paused", tone: "paused" };
  }
  if (instance?.status === "healthy") {
    return { label: "Ready", tone: "ready" };
  }
  return { label: "Ready", tone: "ready" };
}

function transformationSteps(title: string) {
  return [
    "Preparing storage",
    "Checking permissions",
    "Opening local access",
    `Starting ${title}`
  ];
}

async function playTransformationSequence(
  onStep: (step: number) => void,
  signal: AbortSignal
) {
  for (let step = 1; step < transformationStepCount; step += 1) {
    await waitForTransformation(transformationStepDelay, signal);
    if (signal.aborted) return;
    onStep(step);
  }
  await waitForTransformation(transformationStepDelay, signal);
}

function waitForTransformation(milliseconds: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }
    const timer = window.setTimeout(finish, milliseconds);
    signal.addEventListener("abort", cancel, { once: true });

    function finish() {
      signal.removeEventListener("abort", cancel);
      resolve();
    }

    function cancel() {
      window.clearTimeout(timer);
      resolve();
    }
  });
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
