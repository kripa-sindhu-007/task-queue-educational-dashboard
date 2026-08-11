import Link from "next/link";
import { Gamepad2, GraduationCap, ArrowRight } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface Entry {
  href: string;
  icon: LucideIcon;
  eyebrow: string;
  title: string;
  body: string;
  cta: string;
}

const entries: Entry[] = [
  {
    href: "/playground",
    icon: Gamepad2,
    eyebrow: "live dashboard",
    title: "Playground",
    body: "Submit tasks and watch the queue run in real time — leases, retries, node death and recovery, all on screen.",
    cta: "Open Playground",
  },
  {
    href: "/learn",
    icon: GraduationCap,
    eyebrow: "chapter theory",
    title: "Learn",
    body: "Work through the concepts chapter by chapter — delivery guarantees, atomicity, heartbeats and ownership.",
    cta: "Start Learning",
  },
];

export default function EntryCards() {
  return (
    <section className="mx-auto max-w-6xl px-4 py-16 sm:py-20">
      <div className="mb-10 max-w-2xl">
        <p className="font-mono text-xs uppercase tracking-[0.12em] text-primary">
          Where to start
        </p>
        <h2 className="mt-2 text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
          Two ways in
        </h2>
      </div>

      <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
        {entries.map(({ href, icon: Icon, eyebrow, title, body, cta }) => (
          <Link
            key={href}
            href={href}
            className="group flex cursor-pointer flex-col rounded-xl border border-border bg-card p-7 transition-colors hover:border-primary/50 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <span className="flex h-12 w-12 items-center justify-center rounded-lg border border-border bg-background text-primary transition-colors group-hover:border-primary/50">
              <Icon className="h-6 w-6" aria-hidden="true" />
            </span>
            <p className="mt-5 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
              {eyebrow}
            </p>
            <h3 className="mt-1 text-xl font-bold tracking-tight text-foreground">
              {title}
            </h3>
            <p className="mt-2 flex-1 text-sm leading-relaxed text-muted-foreground">
              {body}
            </p>
            <span className="mt-6 inline-flex items-center gap-2 text-sm font-semibold text-primary">
              {cta}
              <ArrowRight
                className="h-4 w-4 transition-transform group-hover:translate-x-0.5"
                aria-hidden="true"
              />
            </span>
          </Link>
        ))}
      </div>
    </section>
  );
}
