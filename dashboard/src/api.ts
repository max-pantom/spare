import type { APIError, Event, Instance, Machine, Recipe } from "./types";

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
    const message =
      body?.error.message ?? `Spare returned HTTP ${response.status}.`;
    const hint = body?.error.hint;
    throw new Error(hint ? `${message} ${hint}` : message);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export async function loadDashboard() {
  const [machine, recipes, instances, events] = await Promise.all([
    request<Machine>("/machine"),
    request<Recipe[]>("/recipes"),
    request<Instance[]>("/instances"),
    request<Event[]>("/events?limit=20")
  ]);
  return { machine, recipes, instances, events };
}

export function startInstance(id: string) {
  return request<Instance>(`/instances/${encodeURIComponent(id)}/start`, {
    method: "POST"
  });
}

export function stopInstance(id: string) {
  return request<Instance>(`/instances/${encodeURIComponent(id)}/stop`, {
    method: "POST"
  });
}
