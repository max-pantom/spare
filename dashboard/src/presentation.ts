import type { Event, Instance } from "./types";

export type PresentedEvent = {
  summary: string;
  detail?: string;
};

export function presentEvent(event: Event, jobTitle: string): PresentedEvent {
  const itemName = detailString(event, "itemName") || detailString(event, "name");
  const source =
    detailString(event, "deviceName") ||
    detailString(event, "sourceName") ||
    detailString(event, "sender");

  switch (event.kind) {
    case "instance_created":
    case "instance_started":
      return { summary: `${jobTitle} started` };
    case "drop_file_received":
    case "file_received":
      return {
        summary: `${itemName || "File"} received${source ? ` from ${source}` : ""}`
      };
    case "address_changed":
    case "instance_address_changed":
    case "port_changed":
      return {
        summary: "Local address changed",
        detail: detailString(event, "address") || detailString(event, "url")
      };
    case "instance_recovered":
    case "worker_recovered":
      return { summary: `${jobTitle} recovered after closing unexpectedly` };
    case "selected_folder_unavailable":
    case "drop_folder_unavailable":
    case "storage_disconnected":
      return { summary: "Storage folder disconnected" };
    case "worker_exited":
      return {
        summary: `${jobTitle} stopped unexpectedly`,
        detail: "Spare is trying to restart it."
      };
    case "restart_limit_reached":
      return {
        summary: `${jobTitle} needs attention`,
        detail: "Spare tried to restart it five times."
      };
    case "instance_stopped":
      return { summary: `${jobTitle} stopped` };
    default:
      return { summary: withoutTrailingPeriod(event.message) };
  }
}

export function presentProblem(instance: Instance, jobTitle: string) {
  const code = instance.problem?.code ?? "";
  const storageProblem =
    code.includes("folder") ||
    code.includes("storage") ||
    code === "selected_folder_unavailable";

  if (storageProblem) {
    return {
      title: "The selected folder is no longer available.",
      explanation: "It may have been moved, renamed, or disconnected.",
      storageProblem: true
    };
  }

  if (code === "restart_limit_reached") {
    return {
      title: `${jobTitle} stopped unexpectedly.`,
      explanation:
        "Spare tried to restart it five times, but it continues to fail.",
      storageProblem: false
    };
  }

  return {
    title: withoutTrailingPeriod(
      instance.problem?.summary || `${jobTitle} needs attention`
    ),
    explanation:
      instance.problem?.recovery ||
      "Spare checked the job and kept its files unchanged.",
    storageProblem: false
  };
}

export function displayMachineName(hostname: string) {
  return hostname.replaceAll("-", " ").replace(/\s+/g, " ").trim();
}

function detailString(event: Event, key: string) {
  const value = event.details?.[key];
  return typeof value === "string" ? value.trim() : "";
}

function withoutTrailingPeriod(value: string) {
  return value.trim().replace(/[.]+$/, "");
}
