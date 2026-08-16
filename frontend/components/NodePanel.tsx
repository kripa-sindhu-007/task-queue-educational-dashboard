"use client";

import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getNodes, getLeader } from "@/lib/api";
import { Node } from "@/lib/types";
import { Server, ServerOff, Crown } from "lucide-react";

const MAX_DOTS = 12;

// WorkerSlots visualizes a node's executor goroutines: one dot per capacity slot,
// the first `inFlight` shown as busy (running/cyan, pulsing). This replaces the
// old standalone Worker Pool panel with a per-node view.
function WorkerSlots({ capacity, inFlight }: { capacity: number; inFlight: number }) {
  if (capacity <= 0) return null;
  const busy = Math.max(0, Math.min(inFlight, capacity));
  const shown = Math.min(capacity, MAX_DOTS);

  return (
    <div className="mt-2.5">
      <div className="mb-1 flex items-center justify-between font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
        <span>workers</span>
        <span className="tabular-nums text-foreground/70">
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
                ? "bg-state-running ring-2 ring-state-running/30 animate-pulse"
                : "bg-foreground/15"
            }`}
          />
        ))}
        {capacity > MAX_DOTS && (
          <span className="font-mono text-[10px] text-muted-foreground">
            +{capacity - MAX_DOTS}
          </span>
        )}
      </div>
    </div>
  );
}

function NodeCard({ node, isLeader }: { node: Node; isLeader: boolean }) {
  const alive = node.alive;
  const reduce = useReducedMotion();
  return (
    <motion.div
      layout
      initial={reduce ? false : { opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={reduce ? { opacity: 0 } : { opacity: 0, scale: 0.9 }}
      transition={{ duration: 0.2 }}
      className={`rounded-lg border p-3.5 ${
        isLeader && alive
          ? "border-amber-400/50 bg-amber-400/5 ring-1 ring-amber-400/30"
          : alive
          ? "border-state-succeeded/25 bg-state-succeeded/5"
          : "border-state-dead/25 bg-state-dead/5 opacity-70"
      }`}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          {alive ? (
            <Server className="h-4 w-4 shrink-0 text-state-succeeded" aria-hidden="true" />
          ) : (
            <ServerOff className="h-4 w-4 shrink-0 text-state-dead" aria-hidden="true" />
          )}
          <span
            className={`truncate font-mono text-sm font-semibold ${
              alive ? "text-state-succeeded" : "text-state-dead"
            }`}
            title={node.hostname}
          >
            {node.hostname || node.id}
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {isLeader && alive && (
            <Badge
              variant="outline"
              className="shrink-0 gap-1 border-amber-400/40 bg-amber-400/15 text-[10px] font-semibold text-amber-300"
              title="Elected leader — runs the delayed scheduler + reaper"
            >
              <Crown className="h-3 w-3" aria-hidden="true" />
              leader
            </Badge>
          )}
          {alive ? (
            <Badge variant="success" className="shrink-0 text-[10px]">
              alive
            </Badge>
          ) : (
            <Badge
              variant="outline"
              className="shrink-0 border-state-dead/25 bg-state-dead/15 text-[10px] text-state-dead"
            >
              dead
            </Badge>
          )}
        </div>
      </div>

      {alive ? (
        <WorkerSlots capacity={node.capacity} inFlight={node.in_flight_tasks} />
      ) : (
        <p className="mt-2.5 text-[11px] font-medium text-state-dead/80">
          {node.in_flight_tasks > 0
            ? `reclaiming ${node.in_flight_tasks} task(s)…`
            : "offline — awaiting reap"}
        </p>
      )}

      <div className="mt-2.5 flex items-center justify-between text-xs text-muted-foreground">
        <span title="tasks currently leased by this node">
          in-flight:{" "}
          <span className="font-mono tabular-nums text-foreground">{node.in_flight_tasks}</span>
        </span>
        <span title="executor goroutines">
          cap <span className="font-mono tabular-nums">{node.capacity}</span>
        </span>
      </div>
      <code className="mt-1 block truncate font-mono text-[10px] text-muted-foreground/70" title={node.id}>
        {node.id}
      </code>
    </motion.div>
  );
}

export default function NodePanel() {
  const { data: nodes } = usePolling(getNodes, 1000);
  const { data: leader } = usePolling(getLeader, 1000);
  const leaderId = leader?.leader_id ?? "";

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
          <span className="font-mono text-xs tabular-nums text-muted-foreground">
            {aliveCount} alive · {busyTotal} busy / {sorted.length} nodes
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {sorted.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
            <ServerOff className="h-7 w-7 text-state-dead" aria-hidden="true" />
            <p className="text-sm font-medium text-foreground">
              No worker nodes registered
            </p>
            <p className="max-w-md text-xs text-muted-foreground">
              Start some workers with{" "}
              <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-state-queued">
                docker compose up --scale worker=4
              </code>{" "}
              and they will appear here.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3 md:grid-cols-4">
            <AnimatePresence mode="popLayout">
              {sorted.map((n) => (
                <NodeCard key={n.id} node={n} isLeader={!!leaderId && n.id === leaderId} />
              ))}
            </AnimatePresence>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
