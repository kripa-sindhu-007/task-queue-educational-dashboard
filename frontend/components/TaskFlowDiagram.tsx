"use client";

import { motion } from "framer-motion";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { usePolling } from "@/lib/hooks";
import { getEnhancedMetrics } from "@/lib/api";
import { ArrowRight } from "lucide-react";

function AnimatedNumber({ value }: { value: number }) {
  return (
    <motion.span
      key={value}
      initial={{ y: -10, opacity: 0 }}
      animate={{ y: 0, opacity: 1 }}
      transition={{ type: "spring", stiffness: 300, damping: 25 }}
      className="tabular-nums"
    >
      {value}
    </motion.span>
  );
}

type Tone = "grape" | "sunny" | "sky" | "aqua";

const toneClasses: Record<Tone, { chip: string; text: string }> = {
  grape: { chip: "bg-grape-soft", text: "text-grape-ink" },
  sunny: { chip: "bg-sunny-soft", text: "text-sunny-ink" },
  sky: { chip: "bg-sky-soft", text: "text-sky-ink" },
  aqua: { chip: "bg-aqua-soft", text: "text-aqua-ink" },
};

function FlowStage({
  label,
  count,
  tone,
}: {
  label: string;
  count: number;
  tone: Tone;
}) {
  const c = toneClasses[tone];
  return (
    <div className={`clay-chip flex flex-col items-center gap-1 px-5 py-3 min-w-[104px] ${c.chip}`}>
      <span className={`font-display text-2xl font-bold ${c.text}`}>
        <AnimatedNumber value={count} />
      </span>
      <span className="text-[11px] font-bold text-foreground/60 whitespace-nowrap">
        {label}
      </span>
    </div>
  );
}

function Arrow() {
  return (
    <div className="flex items-center text-primary/50">
      <ArrowRight className="w-5 h-5" strokeWidth={2.5} />
    </div>
  );
}

export default function TaskFlowDiagram() {
  const { data: metrics } = usePolling(getEnhancedMetrics, 3000);

  if (!metrics) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Task Flow Pipeline</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-2.5 overflow-x-auto pb-2 justify-center flex-wrap sm:flex-nowrap">
          <FlowStage label="Submitted" count={metrics.total_submitted} tone="grape" />
          <Arrow />
          <FlowStage label="Delayed Queue" count={metrics.delayed_queue_size} tone="sunny" />
          <Arrow />
          <FlowStage label="Ready Queue" count={metrics.queue_size} tone="sky" />
          <Arrow />
          <FlowStage label="Workers Active" count={metrics.active_workers} tone="aqua" />
          <Arrow />
          <div className="flex flex-col gap-1.5">
            <Badge variant="success" className="justify-center">
              Completed: {metrics.total_processed}
            </Badge>
            <Badge variant="warning" className="justify-center">
              Retried: {metrics.total_retries}
            </Badge>
            <Badge variant="destructive" className="justify-center">
              Dead Letter: {metrics.dead_letter_size}
            </Badge>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
