import { ShieldCheck, KeyRound, HeartPulse, Activity } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface ValueProp {
  icon: LucideIcon;
  label: string;
  title: string;
  body: string;
}

const props: ValueProp[] = [
  {
    icon: ShieldCheck,
    label: "at-least-once",
    title: "At-least-once + idempotency",
    body: "Tasks are never silently dropped. Delivery is retried until acked, and idempotent handling makes reprocessing safe.",
  },
  {
    icon: KeyRound,
    label: "leases",
    title: "Lease-based delivery",
    body: "A worker leases a task for a bounded window. If it never acks, the lease expires and the task becomes available again.",
  },
  {
    icon: HeartPulse,
    label: "recovery",
    title: "Failure detection & recovery",
    body: "Heartbeats detect dead nodes; a reaper reclaims their in-flight work so no task is stranded on a crashed worker.",
  },
  {
    icon: Activity,
    label: "observability",
    title: "Live observability",
    body: "Every lease, retry, and reclaim streams to the dashboard in real time — the internals are the product, not a black box.",
  },
];

export default function ValueProps() {
  return (
    <section
      id="learn-more"
      className="mx-auto max-w-6xl px-4 py-16 sm:py-20 scroll-mt-16"
    >
      <div className="mb-10 max-w-2xl">
        <p className="font-mono text-xs uppercase tracking-[0.12em] text-primary">
          Not a toy
        </p>
        <h2 className="mt-2 text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
          Real distributed-systems concepts
        </h2>
      </div>

      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        {props.map(({ icon: Icon, label, title, body }) => (
          <div
            key={title}
            className="rounded-lg border border-border bg-card p-5 transition-colors hover:border-primary/40"
          >
            <div className="flex items-center gap-3">
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border bg-background text-primary">
                <Icon className="h-4.5 w-4.5" aria-hidden="true" />
              </span>
              <span className="font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
                {label}
              </span>
            </div>
            <h3 className="mt-4 text-base font-semibold text-foreground">
              {title}
            </h3>
            <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
              {body}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}
