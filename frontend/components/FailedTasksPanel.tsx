"use client";

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getFailedTasks } from "@/lib/api";
import { Skull, WifiOff } from "lucide-react";

export default function FailedTasksPanel() {
  const { data: tasks, error } = usePolling(() => getFailedTasks(0, 20), 5000);

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Skull className="h-4 w-4" aria-hidden="true" />
            Failed Tasks (Dead Letter)
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
            <WifiOff className="h-7 w-7 text-state-failed" aria-hidden="true" />
            <p className="text-sm font-medium text-foreground">
              Dead-letter queue unavailable
            </p>
            <p className="max-w-md text-xs text-muted-foreground">
              Could not reach the queue API. Tasks that exhaust every retry will
              be listed here once the backend is running.
            </p>
          </div>
        </CardContent>
      </Card>
    );
  }

  const list = tasks || [];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Skull className="w-4 h-4" />
          Failed Tasks (Dead Letter)
        </CardTitle>
      </CardHeader>
      <CardContent>
        {list.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
            <Skull className="h-7 w-7 text-muted-foreground/60" aria-hidden="true" />
            <p className="text-sm font-medium text-foreground">No dead-lettered tasks</p>
            <p className="max-w-md text-xs text-muted-foreground">
              Tasks that exhaust all retries land here. Submit a failing batch to
              watch the dead-letter queue fill up.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border text-left">
                  <th className="px-2 py-2 font-mono text-[11px] uppercase tracking-wider font-medium text-muted-foreground">Task ID</th>
                  <th className="px-2 py-2 font-mono text-[11px] uppercase tracking-wider font-medium text-muted-foreground">Priority</th>
                  <th className="px-2 py-2 font-mono text-[11px] uppercase tracking-wider font-medium text-muted-foreground">Attempts</th>
                  <th className="px-2 py-2 font-mono text-[11px] uppercase tracking-wider font-medium text-muted-foreground">Reason</th>
                  <th className="px-2 py-2 font-mono text-[11px] uppercase tracking-wider font-medium text-muted-foreground">Failed At</th>
                </tr>
              </thead>
              <tbody>
                {list.map((ft, i) => (
                  <tr
                    key={`${ft.task.id}-${i}`}
                    className="border-b border-border/50 transition-colors hover:bg-muted/40"
                  >
                    <td className="px-2 py-2">
                      <code className="font-mono font-medium text-state-queued">{ft.task.id}</code>
                    </td>
                    <td className="px-2 py-2 tabular-nums">{ft.task.priority}</td>
                    <td className="px-2 py-2 tabular-nums">
                      {ft.task.retries + 1}/{ft.task.max_retries + 1}
                    </td>
                    <td className="px-2 py-2">
                      <Badge variant="destructive">{ft.reason}</Badge>
                    </td>
                    <td className="px-2 py-2 tabular-nums text-muted-foreground">
                      {new Date(ft.failed_at).toLocaleTimeString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
