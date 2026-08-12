"use client";

import { motion, useReducedMotion } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { usePolling } from "@/lib/hooks";
import { getEnhancedMetrics } from "@/lib/api";
import { ArrowRight, WifiOff } from "lucide-react";

function AnimatedNumber({ value }: { value: number }) {
  const reduce = useReducedMotion();
  if (reduce) {
    return <span className="tabular-nums">{value}</span>;
  }
  return (
    <motion.span
      key={value}
      initial={{ y: -10, opacity: 0 }}
      animate={{ y: 0, opacity: 1 }}
      transition={{ type: "spring", stiffness: 300, damping: 25 }}
      className="tabular-nums"
    >
      {value}
    </motion.span>
  );
}

// Each pipeline stage is keyed to a semantic state token; the label is always
// visible so the stage never relies on color alone.
type Tone = "neutral" | "leased" | "queued" | "running" | "succeeded" | "retrying" | "failed";

const toneClasses: Record<Tone, string> = {
  neutral: "border-border bg-muted/40 text-muted-foreground",
  leased: "border-state-leased/25 bg-state-leased/10 text-state-leased",
  queued: "border-state-queued/25 bg-state-queued/10 text-state-queued",
  running: "border-state-running/25 bg-state-running/10 text-state-running",
  succeeded: "border-state-succeeded/25 bg-state-succeeded/10 text-state-succeeded",
  retrying: "border-state-retrying/25 bg-state-retrying/10 text-state-retrying",
  failed: "border-state-failed/25 bg-state-failed/10 text-state-failed",
};

function FlowStage({
  label,
  count,
  tone,
}: {
  label: string;
  count: number;
  tone: Tone;
}) {
  return (
    <div
      className={`flex min-w-[104px] flex-col items-center gap-1 rounded-lg border px-5 py-3 ${toneClasses[tone]}`}
    >
      <span className="text-2xl font-bold text-foreground">
        <AnimatedNumber value={count} />
      </span>
      <span className="font-mono text-[11px] uppercase tracking-wide whitespace-nowrap text-muted-foreground">
        {label}
      </span>
    </div>
  );
}

function OutcomeChip({
  label,
  count,
  tone,
}: {
  label: string;
  count: number;
  tone: Tone;
}) {
  return (
    <div
      className={`flex items-center justify-between gap-3 rounded-md border px-2.5 py-1 ${toneClasses[tone]}`}
    >
      <span className="font-mono text-[11px] uppercase tracking-wide text-muted-foreground">
        {label}
      </span>
      <span className="text-sm font-bold tabular-nums text-foreground">{count}</span>
    </div>
  );
}

function Arrow() {
  return (
    <div className="flex items-center text-muted-foreground/50" aria-hidden="true">
      <ArrowRight className="h-5 w-5" />
    </div>
  );
}

function FlowShell({ children }: { children: React.ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Task Flow Pipeline</CardTitle>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

export default function TaskFlowDiagram() {
  const { data: metrics, error } = usePolling(getEnhancedMetrics, 3000);

  if (error) {
    return (
      <FlowShell>
        <div className="flex flex-col items-center justify-center gap-2 py-8 text-center">
          <WifiOff className="h-7 w-7 text-state-failed" aria-hidden="true" />
          <p className="text-sm font-medium text-foreground">Pipeline offline</p>
          <p className="max-w-md text-xs text-muted-foreground">
            Waiting for the queue API. Submitted tasks will animate through the
            stages here once the backend is running.
          </p>
        </div>
      </FlowShell>
    );
  }

  if (!metrics) {
    return (
      <FlowShell>
        <div className="flex flex-wrap items-center justify-center gap-2.5">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="h-[74px] w-[104px] animate-pulse rounded-lg border border-border bg-muted/40"
            />
          ))}
        </div>
      </FlowShell>
    );
  }

  return (
    <FlowShell>
      <div className="flex flex-wrap items-center justify-center gap-2.5 pb-1 sm:flex-nowrap sm:overflow-x-auto">
        <FlowStage label="Submitted" count={metrics.total_submitted} tone="neutral" />
        <Arrow />
        <FlowStage label="Delayed Queue" count={metrics.delayed_queue_size} tone="leased" />
        <Arrow />
        <FlowStage label="Ready Queue" count={metrics.queue_size} tone="queued" />
        <Arrow />
        <FlowStage label="Workers Active" count={metrics.active_workers} tone="running" />
        <Arrow />
        <div className="flex min-w-[150px] flex-col gap-1.5">
          <OutcomeChip label="Completed" count={metrics.total_processed} tone="succeeded" />
          <OutcomeChip label="Retried" count={metrics.total_retries} tone="retrying" />
          <OutcomeChip label="Dead Letter" count={metrics.dead_letter_size} tone="failed" />
        </div>
      </div>
    </FlowShell>
  );
}
