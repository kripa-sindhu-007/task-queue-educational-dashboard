"use client";

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { usePolling } from "@/lib/hooks";
import { getEnhancedMetrics } from "@/lib/api";
import {
  Activity,
  CheckCircle,
  XCircle,
  RotateCcw,
  Layers,
  Users,
  Clock,
  Skull,
} from "lucide-react";

type Tone =
  | "mint"
  | "coral"
  | "sunny"
  | "sky"
  | "aqua"
  | "grape"
  | "tangerine";

const toneClasses: Record<Tone, { chip: string; text: string }> = {
  mint: { chip: "bg-mint-soft", text: "text-mint-ink" },
  coral: { chip: "bg-coral-soft", text: "text-coral-ink" },
  sunny: { chip: "bg-sunny-soft", text: "text-sunny-ink" },
  sky: { chip: "bg-sky-soft", text: "text-sky-ink" },
  aqua: { chip: "bg-aqua-soft", text: "text-aqua-ink" },
  grape: { chip: "bg-grape-soft", text: "text-grape-ink" },
  tangerine: { chip: "bg-tangerine-soft", text: "text-tangerine-ink" },
};

function StatCard({
  label,
  value,
  icon: Icon,
  tone,
}: {
  label: string;
  value: number | string;
  icon: React.ComponentType<{ className?: string }>;
  tone: Tone;
}) {
  const c = toneClasses[tone];
  return (
    <div className={`clay-chip flex items-center gap-3 p-3 ${c.chip}`}>
      <div className={`shrink-0 ${c.text}`}>
        <Icon className="w-5 h-5" />
      </div>
      <div className="min-w-0">
        <div className={`font-display text-xl font-bold tabular-nums ${c.text}`}>
          {value}
        </div>
        <div className="text-[10px] font-bold text-foreground/60 uppercase tracking-wider">
          {label}
        </div>
      </div>
    </div>
  );
}

export default function MetricsPanel() {
  const { data: metrics, error } = usePolling(getEnhancedMetrics, 3000);

  if (error) {
    return (
      <Card>
        <CardHeader><CardTitle>Metrics</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-coral-ink font-semibold">{error}</p>
        </CardContent>
      </Card>
    );
  }

  if (!metrics) {
    return (
      <Card>
        <CardHeader><CardTitle>Metrics</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Loading...</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader><CardTitle>Metrics</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2.5">
          <StatCard label="Processed" value={metrics.total_processed} icon={CheckCircle} tone="mint" />
          <StatCard label="Failed" value={metrics.total_failed} icon={XCircle} tone="coral" />
          <StatCard label="Retries" value={metrics.total_retries} icon={RotateCcw} tone="sunny" />
          <StatCard label="Queue Size" value={metrics.queue_size} icon={Layers} tone="sky" />
          <StatCard label="Active Workers" value={metrics.active_workers} icon={Users} tone="aqua" />
          <StatCard label="Delayed" value={metrics.delayed_queue_size} icon={Clock} tone="sunny" />
          <StatCard label="Dead Letter" value={metrics.dead_letter_size} icon={Skull} tone="coral" />
          <StatCard label="Submitted" value={metrics.total_submitted} icon={Activity} tone="grape" />
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-foreground/70">Success Rate</span>
            <span className="font-display text-sm font-bold tabular-nums text-mint-ink">
              {metrics.success_rate.toFixed(1)}%
            </span>
          </div>
          <Progress value={metrics.success_rate} />
        </div>
      </CardContent>
    </Card>
  );
}
