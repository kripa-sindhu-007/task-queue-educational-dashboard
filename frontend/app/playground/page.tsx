import TaskSubmissionPanel from "@/components/TaskSubmissionPanel";
import MetricsPanel from "@/components/MetricsPanel";
import FailedTasksPanel from "@/components/FailedTasksPanel";
import TaskFlowDiagram from "@/components/TaskFlowDiagram";
import NodePanel from "@/components/NodePanel";
import CronPanel from "@/components/CronPanel";
import QueuePanel from "@/components/QueuePanel";
import ActivityLog from "@/components/ActivityLog";
import { Boxes } from "lucide-react";

export default function PlaygroundPage() {
  return (
    <div className="max-w-[1400px] mx-auto px-4 py-10 space-y-6">
      <header>
        <div className="flex items-center gap-4">
          <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg border border-border bg-card text-primary">
            <Boxes className="h-6 w-6" aria-hidden="true" />
          </span>
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.12em] text-primary">
              Live cluster
            </p>
            <h1 className="mt-1 text-3xl font-bold tracking-tight text-foreground">
              Task Queue Playground
            </h1>
          </div>
        </div>
        <p className="mt-4 max-w-2xl text-muted-foreground">
          Watch tasks flow through a distributed queue — submit, process, retry,
          and recover in real time.
        </p>
      </header>

      {/* Task Flow Diagram (full width) */}
      <TaskFlowDiagram />

      {/* Submit Form + Enhanced Metrics */}
      <div className="grid grid-cols-1 lg:grid-cols-[360px_1fr] gap-6">
        <TaskSubmissionPanel />
        <MetricsPanel />
      </div>

      {/* Cluster Nodes + Workers (full width) */}
      <NodePanel />

      {/* Scheduled jobs the leader materializes (full width) */}
      <CronPanel />

      {/* Queue Contents + Activity Log */}
      <div className="grid grid-cols-1 lg:grid-cols-[360px_1fr] gap-6">
        <QueuePanel />
        <ActivityLog />
      </div>

      {/* Failed Tasks Table (full width) */}
      <FailedTasksPanel />
    </div>
  );
}
