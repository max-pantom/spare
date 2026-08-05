import type {
  CreateInstanceInput,
  DesktopPreferences,
  DesktopSnapshot,
  Instance,
  JobPackage,
  JobPackageReview,
  JobProfile
} from "./types";

export type DesktopBridge = {
  Bootstrap(): Promise<DesktopSnapshot>;
  Snapshot(): Promise<DesktopSnapshot>;
  CreateInstance(input: CreateInstanceInput): Promise<Instance>;
  SwitchInstance(input: CreateInstanceInput): Promise<Instance>;
  ConfigureInstance(id: string, input: CreateInstanceInput): Promise<Instance>;
  StartInstance(id: string): Promise<Instance>;
  StopInstance(id: string): Promise<Instance>;
  PromoteInstance(id: string): Promise<Instance>;
  RemoveInstance(id: string): Promise<void>;
  Repair(): Promise<DesktopSnapshot>;
  SavePreferences(preferences: DesktopPreferences): Promise<void>;
  ChooseDirectory(purpose: string): Promise<string>;
  ChooseFile(purpose: string): Promise<string>;
  ChooseFiles(purpose: string): Promise<string[]>;
  DescribeDroppedPaths(paths: string[]): Promise<
    Array<{
      path: string;
      name: string;
      kind: "directory" | "recipe-package" | "backup" | "file" | "unsupported";
    }>
  >;
  PendingLaunchPaths(): Promise<string[]>;
  OpenRecipePackage(path: string): Promise<void>;
  ReviewJobPackage(path: string): Promise<JobPackageReview>;
  InstallJobPackage(path: string): Promise<JobPackage>;
  UninstallJobPackage(id: string): Promise<void>;
  JobProfile(id: string): Promise<JobProfile>;
  OpenJobCatalog(): Promise<void>;
  AddDropFiles(instanceId: string, paths: string[]): Promise<string[]>;
  ExportBackup(instanceId: string): Promise<string>;
  RestoreBackup(source: string): Promise<Instance>;
  OpenURL(url: string): Promise<void>;
  OpenDashboard(): Promise<void>;
  RevealPath(path: string): Promise<void>;
  RevealReceivedFile(instanceId: string, name: string): Promise<void>;
  Uninstall(): Promise<void>;
  Quit(): Promise<void>;
};

declare global {
  interface Window {
    go?: {
      desktop?: {
        App?: DesktopBridge;
      };
    };
    runtime?: {
      EventsOn?: (
        name: string,
        callback: (...data: unknown[]) => void
      ) => () => void;
    };
  }
}

export function desktopBridge(): DesktopBridge | undefined {
  return window.go?.desktop?.App;
}
