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
    <span className="font-display text-xs font-bold tabular-nums text-sunny-ink">
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
      className="flex items-center justify-between py-1.5 px-2.5 rounded-xl bg-muted"
    >
      <code className="text-xs font-semibold text-foreground truncate max-w-[140px]">{task.id}</code>
      <Badge variant="info" className="text-[10px]">P{task.priority}</Badge>
    </motion.div>
  );
}

function DelayedTaskRow({ entry }: { entry: DelayedEntry }) {
  return (
    <motion.div
      initial={{ opacity: 0, x: -20 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: 20 }}
      className="flex items-center justify-between py-1.5 px-2.5 rounded-xl bg-muted"
    >
      <code className="text-xs font-semibold text-foreground truncate max-w-[120px]">{entry.task.id}</code>
      <div className="flex items-center gap-2">
        <Badge variant="info" className="text-[10px]">P{entry.task.priority}</Badge>
        <CountdownTimer executeAt={entry.execute_at} />
      </div>
    </motion.div>
  );
}

export default function QueuePanel() {
  const { data: queues } = usePolling(getQueues, 2000);

  if (!queues) return null;

  const ready = queues.ready || [];
  const delayed = queues.delayed || [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Queue Contents</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <div className="flex items-center gap-1.5 mb-2">
            <Zap className="w-4 h-4 text-sky" strokeWidth={2.5} />
            <span className="text-xs font-bold text-foreground/70">Ready Queue ({ready.length})</span>
          </div>
          <div className="space-y-1 max-h-[200px] overflow-y-auto">
            <AnimatePresence mode="popLayout">
              {ready.length === 0 ? (
                <p className="text-xs text-muted-foreground italic px-2">Empty</p>
              ) : (
                ready.map((t) => <ReadyTaskRow key={t.id} task={t} />)
              )}
            </AnimatePresence>
          </div>
        </div>

        <div className="border-t border-border pt-3">
          <div className="flex items-center gap-1.5 mb-2">
            <Clock className="w-4 h-4 text-sunny" strokeWidth={2.5} />
            <span className="text-xs font-bold text-foreground/70">Delayed Queue ({delayed.length})</span>
          </div>
          <div className="space-y-1 max-h-[200px] overflow-y-auto">
            <AnimatePresence mode="popLayout">
              {delayed.length === 0 ? (
                <p className="text-xs text-muted-foreground italic px-2">Empty</p>
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
