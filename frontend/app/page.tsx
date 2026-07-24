import TaskSubmissionPanel from "@/components/TaskSubmissionPanel";
import MetricsPanel from "@/components/MetricsPanel";
import FailedTasksPanel from "@/components/FailedTasksPanel";
import TaskFlowDiagram from "@/components/TaskFlowDiagram";
import NodePanel from "@/components/NodePanel";
import QueuePanel from "@/components/QueuePanel";
import ActivityLog from "@/components/ActivityLog";
import { Boxes } from "lucide-react";

export default function Home() {
  return (
    <div className="max-w-[1400px] mx-auto px-4 py-8 space-y-5">
      <header className="flex items-center gap-4">
        <div className="clay-chip flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-grape text-white float-soft">
          <Boxes className="h-7 w-7" />
        </div>
        <div>
          <h1 className="font-display text-3xl font-bold tracking-tight text-foreground">
            Task Queue Playground
          </h1>
          <p className="text-sm text-muted-foreground font-medium">
            Watch tasks flow through a distributed queue — submit, process, retry, recover.
          </p>
        </div>
      </header>

      {/* Task Flow Diagram (full width) */}
      <TaskFlowDiagram />

      {/* Submit Form + Enhanced Metrics */}
      <div className="grid grid-cols-1 lg:grid-cols-[360px_1fr] gap-5">
        <TaskSubmissionPanel />
        <MetricsPanel />
      </div>

      {/* Cluster Nodes + Workers (full width) */}
      <NodePanel />

      {/* Queue Contents + Activity Log */}
      <div className="grid grid-cols-1 lg:grid-cols-[360px_1fr] gap-5">
        <QueuePanel />
        <ActivityLog />
      </div>

      {/* Failed Tasks Table (full width) */}
      <FailedTasksPanel />
    </div>
  );
}
