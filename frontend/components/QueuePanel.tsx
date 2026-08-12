"use client";

import { useEffect, useState, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getQueues } from "@/lib/api";
import { DelayedEntry, Task } from "@/lib/types";
import { Clock, Zap } from "lucide-react";

function CountdownTimer({ executeAt }: { executeAt: number }) {
  const [remaining, setRemaining] = useState(0);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    const tick = () => {
      const left = Math.max(0, Math.floor(executeAt - Date.now() / 1000));
      setRemaining(left);
    };
    tick();
    intervalRef.current = setInterval(tick, 1000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [executeAt]);

  return (
    <span className="font-mono text-xs font-semibold tabular-nums text-state-leased">
      {remaining}s
    </span>
  );
}

function ReadyTaskRow({ task }: { task: Task }) {
  return (
    <motion.div
      initial={{ opacity: 0, x: -20 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: 20 }}
      className="flex items-center justify-between gap-2 py-1.5 px-2.5 rounded-md bg-muted/60 border border-border/50"
    >
      <code className="font-mono text-xs font-medium text-foreground truncate max-w-[140px]">{task.id}</code>
      <Badge variant="info" className="text-[10px] tabular-nums">P{task.priority}</Badge>
    </motion.div>
  );
}

function DelayedTaskRow({ entry }: { entry: DelayedEntry }) {
  return (
    <motion.div
      initial={{ opacity: 0, x: -20 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: 20 }}
      className="flex items-center justify-between gap-2 py-1.5 px-2.5 rounded-md bg-muted/60 border border-border/50"
    >
      <code className="font-mono text-xs font-medium text-foreground truncate max-w-[120px]">{entry.task.id}</code>
      <div className="flex items-center gap-2">
        <Badge variant="warning" className="text-[10px] tabular-nums">P{entry.task.priority}</Badge>
        <CountdownTimer executeAt={entry.execute_at} />
      </div>
    </motion.div>
  );
}

export default function QueuePanel() {
  const { data: queues } = usePolling(getQueues, 2000);

  const ready = queues?.ready ?? [];
  const delayed = queues?.delayed ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Queue Contents</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <div className="mb-2 flex items-center gap-1.5">
            <Zap className="h-4 w-4 text-state-queued" aria-hidden="true" />
            <span className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
              Ready Queue
            </span>
            <span className="ml-auto font-mono text-xs tabular-nums text-foreground/70">
              {ready.length}
            </span>
          </div>
          <div className="max-h-[200px] space-y-1 overflow-y-auto">
            <AnimatePresence mode="popLayout">
              {ready.length === 0 ? (
                <p className="px-2 py-1 text-xs text-muted-foreground">
                  No tasks ready for a worker.
                </p>
              ) : (
                ready.map((t) => <ReadyTaskRow key={t.id} task={t} />)
              )}
            </AnimatePresence>
          </div>
        </div>

        <div className="border-t border-border pt-3">
          <div className="mb-2 flex items-center gap-1.5">
            <Clock className="h-4 w-4 text-state-leased" aria-hidden="true" />
            <span className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
              Delayed Queue
            </span>
            <span className="ml-auto font-mono text-xs tabular-nums text-foreground/70">
              {delayed.length}
            </span>
          </div>
          <div className="max-h-[200px] space-y-1 overflow-y-auto">
            <AnimatePresence mode="popLayout">
              {delayed.length === 0 ? (
                <p className="px-2 py-1 text-xs text-muted-foreground">
                  No tasks waiting on a delay.
                </p>
              ) : (
                delayed.map((e) => <DelayedTaskRow key={e.task.id} entry={e} />)
              )}
            </AnimatePresence>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
