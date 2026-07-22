"use client";

import { motion, AnimatePresence } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getNodes } from "@/lib/api";
import { Node } from "@/lib/types";
import { Server, ServerOff } from "lucide-react";

function NodeCard({ node }: { node: Node }) {
  const alive = node.alive;
  return (
    <motion.div
      layout
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.9 }}
      transition={{ duration: 0.2 }}
      className={`rounded-lg border p-3 ${
        alive
          ? "border-emerald-500/40 bg-emerald-500/5"
          : "border-red-500/40 bg-red-500/5 opacity-70"
      }`}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          {alive ? (
            <Server className="w-4 h-4 text-emerald-400 shrink-0" />
          ) : (
            <ServerOff className="w-4 h-4 text-red-400 shrink-0" />
          )}
          <span className="text-sm font-medium truncate" title={node.hostname}>
            {node.hostname || node.id}
          </span>
        </div>
        <Badge variant={alive ? "success" : "destructive"} className="text-[10px] shrink-0">
          {alive ? "alive" : "dead"}
        </Badge>
      </div>

      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
        <span title="tasks currently in flight on this node">
          in-flight: <span className="tabular-nums text-foreground">{node.in_flight_tasks}</span>
        </span>
        <span title="executor goroutines">cap {node.capacity}</span>
      </div>
      <code className="mt-1 block text-[10px] text-muted-foreground truncate" title={node.id}>
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

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span>Cluster Nodes</span>
          <span className="text-xs font-normal text-muted-foreground tabular-nums">
            {aliveCount} alive / {sorted.length} total
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {sorted.length === 0 ? (
          <p className="text-sm text-muted-foreground">No worker nodes registered.</p>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
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
