import TaskSubmissionPanel from "@/components/TaskSubmissionPanel";
import MetricsPanel from "@/components/MetricsPanel";
import FailedTasksPanel from "@/components/FailedTasksPanel";
import TaskFlowDiagram from "@/components/TaskFlowDiagram";
import NodePanel from "@/components/NodePanel";
import WorkerPoolPanel from "@/components/WorkerPoolPanel";
import QueuePanel from "@/components/QueuePanel";
import ActivityLog from "@/components/ActivityLog";

export default function Home() {
  return (
    <div className="max-w-[1400px] mx-auto px-4 py-6 space-y-4">
      <div>
        <h1 className="text-2xl font-bold">Task Queue Dashboard</h1>
        <p className="text-sm text-muted-foreground">
          An educational view into distributed task processing
        </p>
      </div>

      {/* Row 1: Task Flow Diagram (full width) */}
      <TaskFlowDiagram />

      {/* Row 2: Submit Form + Enhanced Metrics */}
      <div className="grid grid-cols-1 lg:grid-cols-[350px_1fr] gap-4">
        <TaskSubmissionPanel />
        <MetricsPanel />
      </div>

      {/* Row 3: Cluster Nodes + Worker Pool (full width) */}
      <NodePanel />
      <WorkerPoolPanel />

      {/* Row 4: Queue Contents + Activity Log */}
      <div className="grid grid-cols-1 lg:grid-cols-[350px_1fr] gap-4">
        <QueuePanel />
        <ActivityLog />
      </div>

      {/* Row 5: Failed Tasks Table (full width) */}
      <FailedTasksPanel />
    </div>
  );
}
