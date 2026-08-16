export interface Task {
  id: string;
  priority: number;
  delay: number;
  max_retries: number;
  retries: number;
  status: "pending" | "processing" | "completed" | "failed";
  created_at: string;
  error?: string;
}

export interface Metrics {
  total_processed: number;
  total_failed: number;
  total_retries: number;
  queue_size: number;
  active_workers: number;
}

export interface FailedTask {
  task: Task;
  failed_at: string;
  reason: string;
}

export interface SubmitTaskRequest {
  id: string;
  priority: number;
  delay: number;
  max_retries: number;
}

export interface TaskEvent {
  id: string;
  task_id: string;
  type: "submitted" | "started" | "completed" | "failed" | "retrying" | "dead_lettered" | "promoted" | "reclaimed" | "node_joined" | "node_dead" | "redriven";
  worker_id: number;
  detail: string;
  timestamp: string;
}

export interface WorkerState {
  id: number;
  status: "idle" | "processing";
  task_id?: string;
  started_at?: string;
}

// Node is a standalone worker process in the cluster (Phase 2). Presence is
// tracked via heartbeat TTL; `alive=false` means the heartbeat expired but the
// reaper has not yet cleaned it up.
export interface Node {
  id: string;
  hostname: string;
  capacity: number;
  started_at: string;
  alive: boolean;
  in_flight_tasks: number;
  role?: string; // "worker" | "server" (P4); absent on older payloads
}

// P4 leader election: which node currently holds the taskqueue:leader lease.
export interface LeaderInfo {
  leader_id: string;
  is_self: boolean;
}

// P4.4 cron jobs: a recurring schedule the leader materializes into tasks.
export interface CronTaskTemplate {
  type: string;
  payload?: Record<string, unknown>;
  priority: number;
  max_retries: number;
}

export interface CronJob {
  id: string;
  schedule: string;
  task: CronTaskTemplate;
  enabled: boolean;
  created_at: string;
  last_scheduled_unix: number;
}

export interface CreateCronRequest {
  id?: string;
  schedule: string;
  task: CronTaskTemplate;
  enabled: boolean;
}

export interface DelayedEntry {
  task: Task;
  execute_at: number;
}

export interface QueueSnapshot {
  ready: Task[];
  delayed: DelayedEntry[];
}

export interface EnhancedMetrics extends Metrics {
  success_rate: number;
  delayed_queue_size: number;
  dead_letter_size: number;
  total_submitted: number;
}
