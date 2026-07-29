import type { DesktopBridge } from "./desktop";
import type {
  CreateInstanceInput,
  DesktopPreferences,
  DesktopSnapshot,
  Event,
  Instance,
  Machine,
  Recipe
} from "./types";

const machine: Machine = {
  id: "spare_preview",
  hostname: "preview-mac",
  os: "darwin",
  architecture: "arm64",
  logicalCores: 8,
  memoryTotalBytes: 17_179_869_184,
  storageAvailableBytes: 214_748_364_800,
  lanAddresses: ["192.168.1.24"],
  capabilities: {
    canServeLAN: true,
    canRunPersistent: true,
    canStoreLargeFiles: true,
    canRunContainers: false,
    hasBattery: true,
    hasExternalStorage: false
  },
  initializedAt: "2026-07-26T10:00:00Z",
  lastProfiledAt: "2026-07-26T10:00:00Z"
};

const recipes: Recipe[] = [
  {
    id: "drop",
    title: "Drop",
    version: "0.1.0",
    description: "Send files to this computer from a browser on the local network.",
    runtime: "native",
    supportedSystems: ["darwin", "windows", "linux"],
    resources: {
      memoryRecommendedBytes: 134_217_728,
      memoryMaximumBytes: 536_870_912,
      cpuMaximum: 1
    },
    config: [
      {
        id: "destination",
        type: "directory",
        label: "Destination folder",
        description: "Files received through Drop are written here.",
        required: true
      },
      {
        id: "max-file-size",
        type: "size",
        label: "Maximum file size",
        description: "Drop rejects individual files larger than this limit.",
        required: false,
        default: "2GB"
      }
    ],
    permissions: [
      {
        id: "network.local",
        description: "Accept connections from your local network",
        granted: true
      },
      {
        id: "network.internet",
        description: "Access internet services",
        granted: false
      }
    ],
    compatibility: {
      supported: true,
      rating: "Excellent",
      reasons: ["This computer has enough memory."],
      warnings: ["This computer may sleep when its lid is closed."]
    }
  },
  {
    id: "site",
    title: "Site",
    version: "0.1.0",
    description: "Serve a folder as a read-only website on the local network.",
    runtime: "native",
    supportedSystems: ["darwin", "windows", "linux"],
    resources: {
      memoryRecommendedBytes: 67_108_864,
      memoryMaximumBytes: 268_435_456,
      cpuMaximum: 1
    },
    config: [
      {
        id: "path",
        type: "directory",
        label: "Site folder",
        description: "The folder Spare will serve read-only.",
        required: true
      }
    ],
    permissions: [
      {
        id: "filesystem.read",
        description: "Read files in the folder you select",
        granted: true
      }
    ],
    compatibility: {
      supported: true,
      rating: "Excellent",
      reasons: ["This computer has enough memory."],
      warnings: []
    }
  },
  {
    id: "hook",
    title: "Hook",
    version: "0.1.0",
    description: "Receive, inspect, and replay webhook requests locally.",
    runtime: "native",
    supportedSystems: ["darwin", "windows", "linux"],
    resources: {
      memoryRecommendedBytes: 67_108_864,
      memoryMaximumBytes: 268_435_456,
      cpuMaximum: 1
    },
    config: [],
    permissions: [
      {
        id: "network.local",
        description: "Accept connections from your local network",
        granted: true
      }
    ],
    compatibility: {
      supported: true,
      rating: "Excellent",
      reasons: ["This computer has enough memory."],
      warnings: []
    }
  }
];

const previewDrop: Instance = {
  id: "drop",
  recipeId: "drop",
  version: "0.1.0",
  runtime: "native",
  mode: "installed",
  desiredState: "running",
  status: "healthy",
  rootPath: "/Users/you/Downloads/Spare",
  dataPath: "/Users/you/Downloads/Spare",
  config: {
    destination: "/Users/you/Downloads/Spare",
    "max-file-size": 2_000_000_000
  },
  port: 7340,
  portMode: "auto",
  urls: [
    "http://127.0.0.1:7340",
    "http://192.168.1.24:7340",
    "http://preview-mac.local:7340"
  ],
  storageAvailableBytes: 107_374_182_400,
  itemCount: 7,
  createdAt: "2026-07-26T10:00:00Z",
  updatedAt: "2026-07-26T10:00:02Z"
};

const initialPreferences: DesktopPreferences = {
  theme: "dark",
  notifications: true,
  recipeNotifications: {
    drop: true,
    site: true,
    hook: true
  },
  openAfterLogin: false,
  showInMenuBar: true,
  keepRecipesRunningAfterLogin: true
};

const previewEvents: Event[] = [
  {
    id: 1,
    instanceId: "drop",
    level: "info",
    kind: "drop_file_received",
    message: "report.pdf was received.",
    details: { itemName: "report.pdf" },
    createdAt: "2026-07-26T10:00:03Z"
  },
  {
    id: 2,
    instanceId: "drop",
    level: "info",
    kind: "instance_healthy",
    message: "Drop is ready.",
    createdAt: "2026-07-26T10:00:02Z"
  },
  {
    id: 3,
    instanceId: "drop",
    level: "info",
    kind: "instance_started",
    message: "Drop started on the local network.",
    createdAt: "2026-07-26T09:59:58Z"
  }
];

export function createDashboardPreviewState() {
  return structuredClone({
    machine,
    recipes,
    instances: [previewDrop],
    events: previewEvents
  });
}

export function createDesktopPreviewBridge(): DesktopBridge {
  let instances: Instance[] = [structuredClone(previewDrop)];
  let events: Event[] = structuredClone(previewEvents);
  let preferences = structuredClone(initialPreferences);
  let nextEventID = 3;
  const previewState = new URLSearchParams(window.location.search).get(
    "desktop-state"
  );

  const snapshot = (): DesktopSnapshot =>
    structuredClone({
      surface: "desktop",
      machine,
      recipes,
      instances,
      events,
      preferences
    });
  const pendingStartup = new Promise<DesktopSnapshot>(() => undefined);
  const startupSnapshot = (): Promise<DesktopSnapshot> => {
    if (previewState === "loading") return pendingStartup;
    if (previewState === "unavailable") {
      return Promise.reject(
        new Error("Spare could not connect to its background service.")
      );
    }
    return Promise.resolve(snapshot());
  };

  const currentInstance = (id: string): Instance => {
    const instance = instances.find((candidate) => candidate.id === id);
    if (!instance) throw new Error("The preview instance is unavailable.");
    return instance;
  };

  const saveInstance = (next: Instance): Instance => {
    instances = [next];
    return structuredClone(next);
  };

  const addEvent = (
    instanceId: string,
    kind: string,
    message: string,
    details?: Record<string, unknown>
  ) => {
    events = [
      {
        id: nextEventID++,
        instanceId,
        level: "info",
        kind,
        message,
        details,
        createdAt: new Date().toISOString()
      },
      ...events
    ];
  };

  const createInstance = (input: CreateInstanceInput): Instance => {
    const selectedPath = String(
      input.config.destination ?? input.config.path ?? ""
    );
    const next: Instance = {
      ...structuredClone(previewDrop),
      id: input.recipeId,
      recipeId: input.recipeId,
      mode: input.mode,
      rootPath: selectedPath,
      dataPath: selectedPath,
      config: structuredClone(input.config),
      port: input.port || 7340,
      portMode: input.portMode,
      itemCount: input.recipeId === "drop" ? 0 : 0,
      storageAvailableBytes:
        input.recipeId === "drop" ? previewDrop.storageAvailableBytes : 0,
      updatedAt: new Date().toISOString()
    };
    instances = [next];
    addEvent(next.id, "instance_created", `${titleFor(next.recipeId)} started.`);
    return structuredClone(next);
  };

  return {
    Bootstrap: startupSnapshot,
    Snapshot: startupSnapshot,
    CreateInstance: async (input) => createInstance(input),
    ConfigureInstance: async (id, input) => {
      const current = currentInstance(id);
      const selectedPath = String(
        input.config.destination ?? input.config.path ?? current.dataPath
      );
      return saveInstance({
        ...current,
        mode: input.mode,
        rootPath: selectedPath,
        dataPath: selectedPath,
        config: structuredClone(input.config),
        port: input.port || current.port,
        portMode: input.portMode,
        updatedAt: new Date().toISOString()
      });
    },
    StartInstance: async (id) => {
      const current = currentInstance(id);
      addEvent(id, "instance_started", `${titleFor(current.recipeId)} started.`);
      return saveInstance({
        ...current,
        desiredState: "running",
        status: "healthy",
        updatedAt: new Date().toISOString()
      });
    },
    StopInstance: async (id) => {
      const current = currentInstance(id);
      addEvent(id, "instance_stopped", `${titleFor(current.recipeId)} paused.`);
      return saveInstance({
        ...current,
        desiredState: "stopped",
        status: "stopped",
        updatedAt: new Date().toISOString()
      });
    },
    PromoteInstance: async (id) => {
      const current = currentInstance(id);
      return saveInstance({ ...current, mode: "installed" });
    },
    RemoveInstance: async () => {
      instances = [];
    },
    Repair: async () => snapshot(),
    SavePreferences: async (next) => {
      preferences = structuredClone(next);
    },
    ChooseDirectory: async () => "/Users/you/Downloads/Spare Preview",
    ChooseFile: async () => "/Users/you/Desktop/drop.spare-backup",
    ChooseFiles: async () => ["/Users/you/Desktop/preview-file.pdf"],
    DescribeDroppedPaths: async (paths) =>
      paths.map((path) => ({
        path,
        name: path.split("/").at(-1) ?? path,
        kind: "file" as const
      })),
    PendingLaunchPaths: async () => [],
    OpenRecipePackage: async () => undefined,
    AddDropFiles: async (id, paths) => {
      const current = currentInstance(id);
      saveInstance({
        ...current,
        itemCount: current.itemCount + paths.length,
        updatedAt: new Date().toISOString()
      });
      const names = paths.map((path) => path.split("/").at(-1) ?? path);
      addEvent(id, "drop_file_received", `${names[0]} was received.`, {
        count: names.length,
        itemName: names[0]
      });
      return names;
    },
    ExportBackup: async () => "/Users/you/Desktop/drop.spare-backup",
    RestoreBackup: async () =>
      instances[0] ?? saveInstance(structuredClone(previewDrop)),
    OpenURL: async () => undefined,
    OpenDashboard: async () => undefined,
    RevealPath: async () => undefined,
    RevealReceivedFile: async () => undefined,
    Uninstall: async () => undefined,
    Quit: async () => undefined
  };
}

function titleFor(recipeId: string): string {
  return recipes.find((recipe) => recipe.id === recipeId)?.title ?? recipeId;
}
