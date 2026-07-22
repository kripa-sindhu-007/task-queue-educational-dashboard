"use client";

import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getEvents } from "@/lib/api";
import { TaskEvent } from "@/lib/types";
import { Terminal } from "lucide-react";

const eventVariantMap: Record<string, "success" | "destructive" | "warning" | "info" | "secondary" | "default"> = {
  submitted: "default",
  started: "info",
  completed: "success",
  failed: "destructive",
  retrying: "warning",
  dead_lettered: "destructive",
  promoted: "secondary",
  reclaimed: "warning",
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
        <span className="text-blue-300">{event.task_id}</span>
        {event.worker_id >= 0 && (
          <span className="text-muted-foreground"> W{event.worker_id}</span>
        )}
        <span className="text-muted-foreground"> — {event.detail}</span>
      </span>
    </motion.div>
  );
}

export default function ActivityLog() {
  const { data: events } = usePolling(() => getEvents(80), 1000);
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
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Terminal className="w-4 h-4" />
            Activity Log
          </CardTitle>
          <span className="text-[10px] text-muted-foreground">
            {events ? `${events.length} events` : "loading..."}
          </span>
        </div>
      </CardHeader>
      <CardContent>
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="activity-log"
        >
          <AnimatePresence initial={false}>
            {events?.map((ev) => (
              <EventRow key={ev.id} event={ev} />
            ))}
          </AnimatePresence>
          {(!events || events.length === 0) && (
            <p className="text-xs text-muted-foreground italic">
              No events yet. Submit some tasks!
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
