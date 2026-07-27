export type Capabilities = {
  canServeLAN: boolean;
  canRunPersistent: boolean;
  canStoreLargeFiles: boolean;
  canRunContainers: boolean;
  hasBattery: boolean;
  hasExternalStorage: boolean;
};

export type Machine = {
  id: string;
  hostname: string;
  os: string;
  architecture: string;
  logicalCores: number;
  memoryTotalBytes: number;
  storageAvailableBytes: number;
  lanAddresses: string[];
  capabilities: Capabilities;
  initializedAt: string;
  lastProfiledAt: string;
};

export type Compatibility = {
  supported: boolean;
  rating: string;
  reasons: string[];
  warnings: string[];
};

export type Recipe = {
  id: string;
  title: string;
  version: string;
  description: string;
  runtime: "native" | "process";
  supportedSystems: string[];
  resources: {
    memoryRecommendedBytes: number;
    memoryMaximumBytes: number;
    cpuMaximum: number;
  };
  config: Array<{
    id: string;
    type: string;
    label: string;
    description?: string;
    required: boolean;
    default?: unknown;
  }>;
  permissions: Array<{
    id: string;
    description: string;
    granted: boolean;
  }>;
  compatibility: Compatibility;
};

export type Problem = {
  code: string;
  severity: "warning" | "error";
  summary: string;
  recovery: string;
};

export type Instance = {
  id: string;
  recipeId: string;
  version: string;
  runtime: "native" | "process";
  mode: "temporary" | "installed";
  desiredState: "running" | "stopped";
  status: "starting" | "healthy" | "degraded" | "stopped" | "failed" | "removing";
  rootPath: string;
  dataPath: string;
  config: Record<string, unknown>;
  port: number;
  portMode: "auto" | "fixed";
  urls: string[];
  storageAvailableBytes: number;
  itemCount: number;
  startedAt?: string;
  createdAt: string;
  updatedAt: string;
  problem?: Problem;
};

export type Event = {
  id: number;
  instanceId?: string;
  level: "info" | "warning" | "error";
  kind: string;
  message: string;
  details?: Record<string, unknown>;
  createdAt: string;
};

export type APIError = {
  error: {
    code: string;
    message: string;
    hint?: string;
  };
};
