import {
  Metrics,
  FailedTask,
  SubmitTaskRequest,
  Task,
  TaskEvent,
  WorkerState,
  Node,
  QueueSnapshot,
  EnhancedMetrics,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function submitTask(req: SubmitTaskRequest): Promise<Task> {
  const res = await fetch(`${API_BASE}/api/tasks`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || "Failed to submit task");
  }
  return res.json();
}

export async function getMetrics(): Promise<Metrics> {
  const res = await fetch(`${API_BASE}/api/metrics`);
  if (!res.ok) throw new Error("Failed to fetch metrics");
  return res.json();
}

export async function getFailedTasks(
  offset = 0,
  limit = 20
): Promise<FailedTask[]> {
  const res = await fetch(
    `${API_BASE}/api/tasks/failed?offset=${offset}&limit=${limit}`
  );
  if (!res.ok) throw new Error("Failed to fetch failed tasks");
  return res.json();
}

export async function getEvents(limit = 50): Promise<TaskEvent[]> {
  const res = await fetch(`${API_BASE}/api/events?limit=${limit}`);
  if (!res.ok) throw new Error("Failed to fetch events");
  return res.json();
}

export async function getClusterEvents(limit = 50): Promise<TaskEvent[]> {
  const res = await fetch(`${API_BASE}/api/events/cluster?limit=${limit}`);
  if (!res.ok) throw new Error("Failed to fetch cluster events");
  return res.json();
}

export async function getWorkers(): Promise<WorkerState[]> {
  const res = await fetch(`${API_BASE}/api/workers`);
  if (!res.ok) throw new Error("Failed to fetch workers");
  return res.json();
}

export async function getNodes(): Promise<Node[]> {
  const res = await fetch(`${API_BASE}/api/nodes`);
  if (!res.ok) throw new Error("Failed to fetch nodes");
  return res.json();
}

export async function getQueues(): Promise<QueueSnapshot> {
  const res = await fetch(`${API_BASE}/api/queues`);
  if (!res.ok) throw new Error("Failed to fetch queues");
  return res.json();
}

export async function getEnhancedMetrics(): Promise<EnhancedMetrics> {
  const res = await fetch(`${API_BASE}/api/metrics/enhanced`);
  if (!res.ok) throw new Error("Failed to fetch enhanced metrics");
  return res.json();
}

export async function flushData(): Promise<void> {
  const res = await fetch(`${API_BASE}/api/flush`, { method: "DELETE" });
  if (!res.ok) throw new Error("Failed to flush data");
}
