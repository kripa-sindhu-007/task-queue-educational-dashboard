"use client";

import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getEvents, getClusterEvents } from "@/lib/api";
import { TaskEvent } from "@/lib/types";
import { Terminal } from "lucide-react";
import { cn } from "@/lib/utils";

type LogMode = "all" | "cluster";

const eventVariantMap: Record<string, "success" | "destructive" | "warning" | "info" | "secondary" | "default"> = {
  submitted: "default",
  started: "info",
  completed: "success",
  failed: "destructive",
  retrying: "warning",
  dead_lettered: "destructive",
  promoted: "secondary",
  reclaimed: "warning",
  node_joined: "success",
  node_dead: "destructive",
  redriven: "info",
};

function EventRow({ event }: { event: TaskEvent }) {
  const time = new Date(event.timestamp).toLocaleTimeString();
  const variant = eventVariantMap[event.type] || "secondary";

  return (
    <motion.div
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0 }}
      className="flex items-start gap-2 py-1 border-b border-border/50 last:border-0"
    >
      <span className="text-[10px] text-muted-foreground shrink-0 tabular-nums mt-0.5">
        {time}
      </span>
      <Badge variant={variant} className="text-[10px] shrink-0">
        {event.type}
      </Badge>
      <span className="text-xs text-foreground truncate">
        <span className="font-mono font-medium text-state-queued">{event.task_id}</span>
        {event.worker_id >= 0 && (
          <span className="text-muted-foreground"> W{event.worker_id}</span>
        )}
        <span className="text-muted-foreground"> — {event.detail}</span>
      </span>
    </motion.div>
  );
}

export default function ActivityLog() {
  const [mode, setMode] = useState<LogMode>("all");
  const { data: events } = usePolling(
    () => (mode === "cluster" ? getClusterEvents(80) : getEvents(80)),
    1000
  );
  const scrollRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = 0;
    }
  }, [events, autoScroll]);

  const handleScroll = () => {
    if (!scrollRef.current) return;
    const { scrollTop } = scrollRef.current;
    setAutoScroll(scrollTop < 10);
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-3">
          <CardTitle className="flex items-center gap-2">
            <Terminal className="w-4 h-4" />
            Activity Log
          </CardTitle>
          <div className="flex items-center gap-3">
            <div
              role="tablist"
              aria-label="Activity log filter"
              className="flex items-center gap-1 rounded-full border border-white/[0.06] bg-muted/30 p-0.5 backdrop-blur"
            >
              {([
                ["all", "All"],
                ["cluster", "Cluster"],
              ] as const).map(([value, label]) => {
                const active = mode === value;
                return (
                  <button
                    key={value}
                    type="button"
                    role="tab"
                    aria-selected={active}
                    onClick={() => setMode(value)}
                    className={cn(
                      "cursor-pointer rounded-full px-2.5 py-0.5 text-[11px] font-medium transition-all",
                      "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                      active
                        ? "bg-primary text-primary-foreground shadow-[0_2px_10px_-2px_var(--color-primary)]"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
            <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
              {events ? `${events.length} events` : "loading…"}
            </span>
          </div>
        </div>
        {mode === "cluster" && (
          <p className="mt-1 text-[11px] text-muted-foreground">
            Cluster lifecycle events — nodes joining, dying, and tasks reclaimed
          </p>
        )}
      </CardHeader>
      <CardContent>
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="activity-log"
        >
          <AnimatePresence initial={false}>
            {(() => {
              // Backend event ids can collide (evt-<nanotimestamp> when several
              // events fire in the same tick), so disambiguate duplicates into
              // stable, unique keys — deterministic per render order.
              const seen = new Map<string, number>();
              return events?.map((ev) => {
                const n = (seen.get(ev.id) ?? 0) + 1;
                seen.set(ev.id, n);
                return (
                  <EventRow key={n === 1 ? ev.id : `${ev.id}#${n}`} event={ev} />
                );
              });
            })()}
          </AnimatePresence>
          {(!events || events.length === 0) && (
            <div className="flex flex-col items-center justify-center gap-1.5 py-10 text-center">
              <Terminal className="h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
              <p className="font-sans text-xs text-muted-foreground">
                {mode === "cluster"
                  ? "No cluster events yet — kill a node to watch it recover."
                  : "No events yet — submit a task to see the stream."}
              </p>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
