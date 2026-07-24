"use client";

import { motion, AnimatePresence } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getNodes } from "@/lib/api";
import { Node } from "@/lib/types";
import { Server, ServerOff } from "lucide-react";

const MAX_DOTS = 12;

// WorkerSlots visualizes a node's executor goroutines: one dot per capacity slot,
// the first `inFlight` shown as busy (aqua, pulsing). This replaces the old
// standalone Worker Pool panel with a per-node view.
function WorkerSlots({ capacity, inFlight }: { capacity: number; inFlight: number }) {
  if (capacity <= 0) return null;
  const busy = Math.max(0, Math.min(inFlight, capacity));
  const shown = Math.min(capacity, MAX_DOTS);

  return (
    <div className="mt-2.5">
      <div className="mb-1 flex items-center justify-between text-[10px] font-bold uppercase tracking-wider text-foreground/50">
        <span>workers</span>
        <span className="font-display tabular-nums text-foreground/70">
          {busy}/{capacity} busy
        </span>
      </div>
      <div className="flex flex-wrap gap-1">
        {Array.from({ length: shown }).map((_, i) => (
          <span
            key={i}
            aria-hidden
            className={`h-2.5 w-2.5 rounded-full ${
              i < busy
                ? "bg-aqua shadow-[0_0_0_2px_rgba(255,255,255,0.6)] animate-pulse"
                : "bg-white/70 shadow-[inset_0_1px_2px_rgba(79,70,180,0.25)]"
            }`}
          />
        ))}
        {capacity > MAX_DOTS && (
          <span className="text-[10px] font-bold text-foreground/50">
            +{capacity - MAX_DOTS}
          </span>
        )}
      </div>
    </div>
  );
}

function NodeCard({ node }: { node: Node }) {
  const alive = node.alive;
  return (
    <motion.div
      layout
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.9 }}
      transition={{ duration: 0.2 }}
      className={`clay-chip p-3.5 ${
        alive ? "bg-mint-soft" : "bg-coral-soft opacity-75"
      }`}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          {alive ? (
            <Server className="w-4 h-4 text-mint-ink shrink-0" strokeWidth={2.5} />
          ) : (
            <ServerOff className="w-4 h-4 text-coral-ink shrink-0" strokeWidth={2.5} />
          )}
          <span
            className={`font-display text-sm font-bold truncate ${
              alive ? "text-mint-ink" : "text-coral-ink"
            }`}
            title={node.hostname}
          >
            {node.hostname || node.id}
          </span>
        </div>
        <Badge variant={alive ? "success" : "destructive"} className="text-[10px] shrink-0">
          {alive ? "alive" : "dead"}
        </Badge>
      </div>

      {alive ? (
        <WorkerSlots capacity={node.capacity} inFlight={node.in_flight_tasks} />
      ) : (
        <p className="mt-2.5 text-[11px] font-bold text-coral-ink/80">
          {node.in_flight_tasks > 0
            ? `reclaiming ${node.in_flight_tasks} task(s)…`
            : "offline — awaiting reap"}
        </p>
      )}

      <div className="mt-2.5 flex items-center justify-between text-xs font-semibold text-foreground/60">
        <span title="tasks currently leased by this node">
          in-flight:{" "}
          <span className="font-display tabular-nums text-foreground">{node.in_flight_tasks}</span>
        </span>
        <span title="executor goroutines">cap {node.capacity}</span>
      </div>
      <code className="mt-1 block text-[10px] text-foreground/45 truncate" title={node.id}>
        {node.id}
      </code>
    </motion.div>
  );
}

export default function NodePanel() {
  const { data: nodes } = usePolling(getNodes, 1000);

  // Sort alive-first, then by ID for stable ordering.
  const sorted = [...(nodes ?? [])].sort((a, b) => {
    if (a.alive !== b.alive) return a.alive ? -1 : 1;
    return a.id.localeCompare(b.id);
  });
  const aliveCount = sorted.filter((n) => n.alive).length;
  const busyTotal = sorted.reduce(
    (sum, n) => sum + (n.alive ? Math.min(n.in_flight_tasks, n.capacity || n.in_flight_tasks) : 0),
    0
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between gap-3">
          <span>Cluster Nodes &amp; Workers</span>
          <span className="font-body text-xs font-bold text-muted-foreground tabular-nums">
            {aliveCount} alive · {busyTotal} busy / {sorted.length} nodes
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {sorted.length === 0 ? (
          <p className="text-sm text-muted-foreground font-medium">
            No worker nodes registered. Start some with{" "}
            <code className="text-sky-ink font-semibold">docker compose up --scale worker=4</code>.
          </p>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2.5">
            <AnimatePresence mode="popLayout">
              {sorted.map((n) => (
                <NodeCard key={n.id} node={n} />
              ))}
            </AnimatePresence>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
