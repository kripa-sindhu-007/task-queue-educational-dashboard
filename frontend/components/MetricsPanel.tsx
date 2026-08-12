"use client";

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { usePolling } from "@/lib/hooks";
import { getEnhancedMetrics } from "@/lib/api";
import {
  Activity,
  CheckCircle,
  XCircle,
  RotateCcw,
  Layers,
  Users,
  Clock,
  Skull,
  WifiOff,
} from "lucide-react";

// Categorical tones drawn from the semantic state scale — color is paired with
// an icon + label so meaning never rests on hue alone (dataviz a11y rule).
type Tone = "queued" | "leased" | "running" | "retrying" | "succeeded" | "failed" | "neutral";

const toneClasses: Record<Tone, string> = {
  queued: "border-state-queued/25 bg-state-queued/10 text-state-queued",
  leased: "border-state-leased/25 bg-state-leased/10 text-state-leased",
  running: "border-state-running/25 bg-state-running/10 text-state-running",
  retrying: "border-state-retrying/25 bg-state-retrying/10 text-state-retrying",
  succeeded: "border-state-succeeded/25 bg-state-succeeded/10 text-state-succeeded",
  failed: "border-state-failed/25 bg-state-failed/10 text-state-failed",
  neutral: "border-border bg-muted/40 text-muted-foreground",
};

function StatCard({
  label,
  value,
  icon: Icon,
  tone,
}: {
  label: string;
  value: number | string;
  icon: React.ComponentType<{ className?: string }>;
  tone: Tone;
}) {
  return (
    <div className={`flex items-center gap-3 rounded-lg border p-3 ${toneClasses[tone]}`}>
      <Icon className="h-5 w-5 shrink-0" aria-hidden="true" />
      <div className="min-w-0">
        <div className="text-xl font-bold tabular-nums text-foreground">{value}</div>
        <div className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
          {label}
        </div>
      </div>
    </div>
  );
}

function MetricsShell({ children }: { children: React.ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Metrics</CardTitle>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

export default function MetricsPanel() {
  const { data: metrics, error } = usePolling(getEnhancedMetrics, 3000);

  if (error) {
    return (
      <MetricsShell>
        <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
          <WifiOff className="h-7 w-7 text-state-failed" aria-hidden="true" />
          <p className="text-sm font-medium text-foreground">Metrics unavailable</p>
          <p className="max-w-xs text-xs text-muted-foreground">
            Could not reach the queue API. Start the backend and this panel will
            populate automatically.
          </p>
        </div>
      </MetricsShell>
    );
  }

  if (!metrics) {
    return (
      <MetricsShell>
        <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div
              key={i}
              className="h-[62px] animate-pulse rounded-lg border border-border bg-muted/40"
            />
          ))}
        </div>
      </MetricsShell>
    );
  }

  return (
    <MetricsShell>
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-4">
          <StatCard label="Processed" value={metrics.total_processed} icon={CheckCircle} tone="succeeded" />
          <StatCard label="Failed" value={metrics.total_failed} icon={XCircle} tone="failed" />
          <StatCard label="Retries" value={metrics.total_retries} icon={RotateCcw} tone="retrying" />
          <StatCard label="Queue Size" value={metrics.queue_size} icon={Layers} tone="queued" />
          <StatCard label="Active Workers" value={metrics.active_workers} icon={Users} tone="running" />
          <StatCard label="Delayed" value={metrics.delayed_queue_size} icon={Clock} tone="leased" />
          <StatCard label="Dead Letter" value={metrics.dead_letter_size} icon={Skull} tone="failed" />
          <StatCard label="Submitted" value={metrics.total_submitted} icon={Activity} tone="neutral" />
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
              Success Rate
            </span>
            <span className="text-sm font-bold tabular-nums text-state-succeeded">
              {metrics.success_rate.toFixed(1)}%
            </span>
          </div>
          <Progress
            value={metrics.success_rate}
            className="bg-state-succeeded/15 [&>[data-slot=progress-indicator]]:bg-state-succeeded"
          />
        </div>
      </div>
    </MetricsShell>
  );
}
