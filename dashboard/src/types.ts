export type Machine = {
  id: string;
  hostname: string;
  os: string;
  architecture: string;
  logicalCores: number;
  memoryTotalBytes: number;
  storageAvailableBytes: number;
  lanAddresses: string[];
  initializedAt: string;
  lastProfiledAt: string;
};

export type Problem = {
  code: string;
  severity: "warning" | "error";
  summary: string;
  recovery: string;
};

export type Instance = {
  id: string;
  recipeId: "site";
  mode: "temporary" | "installed";
  desiredState: "running" | "stopped";
  status: "starting" | "healthy" | "degraded" | "stopped" | "failed" | "removing";
  rootPath: string;
  port: number;
  portMode: "auto" | "fixed";
  urls: string[];
  startedAt?: string;
  createdAt: string;
  updatedAt: string;
  problem?: Problem;
};

export type APIError = {
  error: {
    code: string;
    message: string;
    hint?: string;
  };
};

