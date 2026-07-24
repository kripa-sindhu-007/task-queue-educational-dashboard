"use client";

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getFailedTasks } from "@/lib/api";
import { Skull } from "lucide-react";

export default function FailedTasksPanel() {
  const { data: tasks, error } = usePolling(() => getFailedTasks(0, 20), 5000);

  if (error) {
    return (
      <Card>
        <CardHeader><CardTitle>Failed Tasks (Dead Letter)</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-destructive">{error}</p>
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
          <p className="text-xs text-muted-foreground italic">
            No failed tasks yet. Tasks that exhaust all retries end up here.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-2 px-2 text-muted-foreground font-medium">Task ID</th>
                  <th className="text-left py-2 px-2 text-muted-foreground font-medium">Priority</th>
                  <th className="text-left py-2 px-2 text-muted-foreground font-medium">Attempts</th>
                  <th className="text-left py-2 px-2 text-muted-foreground font-medium">Reason</th>
                  <th className="text-left py-2 px-2 text-muted-foreground font-medium">Failed At</th>
                </tr>
              </thead>
              <tbody>
                {list.map((ft, i) => (
                  <tr key={`${ft.task.id}-${i}`} className="border-b border-border/50">
                    <td className="py-2 px-2">
                      <code className="font-semibold text-sky-ink">{ft.task.id}</code>
                    </td>
                    <td className="py-2 px-2">{ft.task.priority}</td>
                    <td className="py-2 px-2">
                      {ft.task.retries + 1}/{ft.task.max_retries + 1}
                    </td>
                    <td className="py-2 px-2">
                      <Badge variant="destructive">{ft.reason}</Badge>
                    </td>
                    <td className="py-2 px-2 text-muted-foreground">
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
