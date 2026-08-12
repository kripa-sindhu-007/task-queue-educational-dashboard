import {
  Network,
  KeySquare,
  Recycle,
  BarChart3,
  ScrollText,
  Workflow,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface Feature {
  icon: LucideIcon;
  caption: string;
  title: string;
  body: string;
}

const features: Feature[] = [
  {
    icon: Network,
    caption: "cluster.membership",
    title: "Nodes",
    body: "See workers join and leave the cluster as heartbeats come and go.",
  },
  {
    icon: KeySquare,
    caption: "queue.leases",
    title: "Leases",
    body: "Track which worker owns which task and when the lease expires.",
  },
  {
    icon: Recycle,
    caption: "reaper.recover",
    title: "Reaper / recovery",
    body: "Watch stranded tasks get reclaimed after a node is declared dead.",
  },
  {
    icon: BarChart3,
    caption: "metrics.live",
    title: "Metrics",
    body: "Throughput, success rate, retries and dead-letters, updating live.",
  },
  {
    icon: ScrollText,
    caption: "events.stream",
    title: "Activity log",
    body: "A streaming, timestamped feed of every state transition.",
  },
  {
    icon: Workflow,
    caption: "task.pipeline",
    title: "Task flow",
    body: "Follow a single task from submit → queue → worker → outcome.",
  },
];

export default function FeatureGrid() {
  return (
    <section className="border-y border-border/60 bg-card/30">
      <div className="mx-auto max-w-6xl px-4 py-16 sm:py-20">
        <div className="mb-10 max-w-2xl">
          <p className="font-mono text-xs uppercase tracking-[0.12em] text-primary">
            Internals, exposed
          </p>
          <h2 className="mt-2 text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
            Everything you can watch
          </h2>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {features.map(({ icon: Icon, caption, title, body }) => (
            <div
              key={title}
              className="group rounded-lg border border-border bg-card p-5 transition-colors hover:border-primary/40"
            >
              <span className="flex h-10 w-10 items-center justify-center rounded-md border border-border bg-background text-primary transition-colors group-hover:border-primary/40">
                <Icon className="h-5 w-5" aria-hidden="true" />
              </span>
              <p className="mt-4 font-mono text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
                {caption}
              </p>
              <h3 className="mt-1 text-base font-semibold text-foreground">
                {title}
              </h3>
              <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
                {body}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
