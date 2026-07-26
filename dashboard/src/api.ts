import type { APIError, Instance, Machine } from "./types";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    credentials: "same-origin",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers
    }
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as APIError | null;
    const message = body?.error.message ?? `Spare returned HTTP ${response.status}.`;
    const hint = body?.error.hint;
    throw new Error(hint ? `${message} ${hint}` : message);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export async function loadDashboard() {
  const [machine, instances] = await Promise.all([
    request<Machine>("/machine"),
    request<Instance[]>("/instances")
  ]);
  return { machine, instance: instances[0] };
}

export function startSite() {
  return request<Instance>("/instances/site/start", { method: "POST" });
}

export function stopSite() {
  return request<Instance>("/instances/site/stop", { method: "POST" });
}

