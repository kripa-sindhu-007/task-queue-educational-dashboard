"use client";

import { useState } from "react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getCronJobs, createCronJob, deleteCronJob } from "@/lib/api";
import { CronJob } from "@/lib/types";
import { CalendarClock, Clock, Trash2, Plus, Cpu, Moon } from "lucide-react";

// Presets so users never need to know cron syntax. Specs are 6-field
// (sec min hour dom mon dow) — the backend parser accepts seconds.
const PRESETS: { label: string; spec: string }[] = [
  { label: "10s", spec: "*/10 * * * * *" },
  { label: "30s", spec: "*/30 * * * * *" },
  { label: "1 min", spec: "0 * * * * *" },
  { label: "5 min", spec: "0 */5 * * * *" },
  { label: "1 hour", spec: "0 0 * * * *" },
];

const TASK_TYPES: { value: string; label: string; icon: typeof Cpu }[] = [
  { value: "hash", label: "hash (CPU work)", icon: Cpu },
  { value: "sleep", label: "sleep (I/O wait)", icon: Moon },
];

// Translate common cron specs to plain English — the educational bit.
function describeSchedule(spec: string): string {
  const p = spec.trim().split(/\s+/);
  const everyN = (f: string) => {
    const m = f.match(/^\*\/(\d+)$/);
    return m ? Number(m[1]) : null;
  };
  if (p.length === 6) {
    const [sec, min, hour, dom, mon, dow] = p;
    const wild = dom === "*" && mon === "*" && dow === "*";
    if (everyN(sec) && min === "*" && hour === "*" && wild) return `every ${everyN(sec)} seconds`;
    if (sec === "0" && everyN(min) && hour === "*" && wild) return `every ${everyN(min)} minutes`;
    if (sec === "0" && min === "0" && everyN(hour) && wild) return `every ${everyN(hour)} hours`;
    if (sec === "0" && min === "*" && hour === "*" && wild) return "every minute";
    if (sec === "0" && min === "0" && hour === "*" && wild) return "every hour";
    if (sec === "0" && min === "0" && hour === "0" && wild) return "every day at midnight";
  }
  if (p.length === 5) {
    const [min, hour] = p;
    if (everyN(min)) return `every ${everyN(min)} minutes`;
    if (min === "*") return "every minute";
    if (min === "0" && everyN(hour)) return `every ${everyN(hour)} hours`;
    if (min === "0" && hour === "*") return "every hour";
  }
  return `custom schedule`;
}

function relTime(unix: number): string {
  if (!unix) return "not yet";
  const s = Math.max(0, Math.floor(Date.now() / 1000) - unix);
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  return `${Math.floor(s / 3600)}h ago`;
}

function JobRow({ job, onDelete }: { job: CronJob; onDelete: (id: string) => void }) {
  return (
    <div className="rounded-lg border border-border bg-card/50 p-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="truncate font-mono text-sm font-semibold text-foreground" title={job.id}>
              {job.id}
            </span>
            <Badge
              variant={job.enabled ? "success" : "outline"}
              className="shrink-0 text-[10px]"
            >
              {job.enabled ? "enabled" : "paused"}
            </Badge>
          </div>
          <p className="mt-0.5 text-xs text-primary">{describeSchedule(job.schedule)}</p>
          <code className="mt-1 block truncate font-mono text-[10px] text-muted-foreground" title={job.schedule}>
            {job.schedule}
          </code>
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 cursor-pointer px-2 text-muted-foreground hover:text-destructive"
          onClick={() => onDelete(job.id)}
          title="Delete cron job"
          aria-label={`Delete ${job.id}`}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
      <div className="mt-2 flex items-center justify-between text-[11px] text-muted-foreground">
        <span className="rounded bg-muted px-1.5 py-0.5 font-mono">{job.task.type}</span>
        <span className="flex items-center gap-1" title="when the leader last materialized an instance">
          <Clock className="h-3 w-3" />
          last fired {relTime(job.last_scheduled_unix)}
        </span>
      </div>
    </div>
  );
}

export default function CronPanel() {
  const { data: jobs } = usePolling(getCronJobs, 2000);

  const [name, setName] = useState("");
  const [schedule, setSchedule] = useState("*/10 * * * * *");
  const [taskType, setTaskType] = useState("hash");
  const [priority, setPriority] = useState(5);
  const [maxRetries, setMaxRetries] = useState(3);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<{ msg: string; type: "success" | "error" } | null>(null);

  const showToast = (msg: string, type: "success" | "error") => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    try {
      const created = await createCronJob({
        id: name.trim() || undefined,
        schedule: schedule.trim(),
        task: { type: taskType, priority, max_retries: maxRetries },
        enabled: true,
      });
      showToast(`Scheduled "${created.id}" — ${describeSchedule(created.schedule)}`, "success");
      setName("");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Failed to schedule", "error");
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteCronJob(id);
      showToast(`Deleted "${id}"`, "success");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  };

  const list = jobs ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <CalendarClock className="h-5 w-5 text-primary" aria-hidden="true" />
          Scheduled Jobs (Cron)
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className="-mt-1 mb-4 max-w-3xl text-xs text-muted-foreground">
          A cron job fires a task automatically on a repeating schedule. The{" "}
          <span className="text-primary">leader</span> node materializes each due instance onto the
          queue — kill the leader and jobs keep firing on the new one, exactly once per slot (no
          duplicates).
        </p>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,340px)_1fr]">
          {/* Create form */}
          <form onSubmit={handleCreate} className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="cron-name">Job name (optional)</Label>
              <Input
                id="cron-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="nightly-report (auto-generated if empty)"
              />
            </div>

            <div className="space-y-1.5">
              <Label>Schedule</Label>
              <div className="flex flex-wrap gap-1.5">
                {PRESETS.map((p) => (
                  <Button
                    key={p.spec}
                    type="button"
                    size="sm"
                    variant={schedule === p.spec ? "default" : "secondary"}
                    className="h-7 cursor-pointer px-2.5 text-xs"
                    onClick={() => setSchedule(p.spec)}
                  >
                    {p.label}
                  </Button>
                ))}
              </div>
              <Input
                aria-label="Cron schedule expression"
                value={schedule}
                onChange={(e) => setSchedule(e.target.value)}
                className="font-mono text-xs"
              />
              <p className="text-[11px] text-primary">
                fires {describeSchedule(schedule)}{" "}
                <span className="text-muted-foreground">· sec min hour day month weekday</span>
              </p>
            </div>

            <div className="space-y-1.5">
              <Label>Task to run</Label>
              <div className="grid grid-cols-2 gap-2">
                {TASK_TYPES.map((t) => {
                  const Icon = t.icon;
                  return (
                    <Button
                      key={t.value}
                      type="button"
                      size="sm"
                      variant={taskType === t.value ? "default" : "secondary"}
                      className="cursor-pointer justify-start gap-1.5 text-xs"
                      onClick={() => setTaskType(t.value)}
                    >
                      <Icon className="h-3.5 w-3.5" />
                      {t.label}
                    </Button>
                  );
                })}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label htmlFor="cron-priority">Priority</Label>
                <Input
                  id="cron-priority"
                  type="number"
                  min={1}
                  max={10}
                  value={priority}
                  onChange={(e) => setPriority(Number(e.target.value))}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="cron-retries">Max Retries</Label>
                <Input
                  id="cron-retries"
                  type="number"
                  min={0}
                  max={10}
                  value={maxRetries}
                  onChange={(e) => setMaxRetries(Number(e.target.value))}
                />
              </div>
            </div>

            <Button type="submit" disabled={busy} className="w-full cursor-pointer">
              <Plus className="h-4 w-4" />
              {busy ? "Scheduling…" : "Schedule Job"}
            </Button>
          </form>

          {/* Jobs list */}
          <div>
            <div className="mb-2 flex items-center justify-between">
              <Label className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
                Active schedules
              </Label>
              <span className="font-mono text-xs tabular-nums text-muted-foreground">
                {list.length} job{list.length === 1 ? "" : "s"}
              </span>
            </div>
            {list.length === 0 ? (
              <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border py-10 text-center">
                <CalendarClock className="h-7 w-7 text-muted-foreground" aria-hidden="true" />
                <p className="text-sm font-medium text-foreground">No scheduled jobs yet</p>
                <p className="max-w-xs text-xs text-muted-foreground">
                  Pick a schedule and a task on the left, then{" "}
                  <span className="text-primary">Schedule Job</span>. Watch instances appear in the
                  Activity Log below.
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                {list
                  .slice()
                  .sort((a, b) => a.id.localeCompare(b.id))
                  .map((job) => (
                    <JobRow key={job.id} job={job} onDelete={handleDelete} />
                  ))}
              </div>
            )}
          </div>
        </div>

        {toast && (
          <div
            className={`toast rounded-lg border px-4 py-2.5 text-sm font-semibold shadow-lg ${
              toast.type === "success"
                ? "border-state-succeeded/25 bg-state-succeeded/15 text-state-succeeded"
                : "border-state-failed/25 bg-state-failed/15 text-state-failed"
            }`}
          >
            {toast.msg}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
