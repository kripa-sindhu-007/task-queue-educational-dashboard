import Link from "next/link";
import { Boxes, ArrowRight, BookOpen, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  QueueTutorialProvider,
  QueueBackground,
} from "@/components/landing/QueueTutorial";

export default function Hero() {
  return (
    <section className="relative min-h-dvh w-full overflow-hidden border-b border-border/60">
      <QueueTutorialProvider>
        {/* z-0 — full-bleed animated lifecycle diagram, filling the whole hero. */}
        <div aria-hidden="true" className="pointer-events-none absolute inset-0 z-0">
          <QueueBackground />
        </div>

        {/* z-0 — ambient glow for depth. */}
        <div aria-hidden="true" className="pointer-events-none absolute inset-0 z-0">
          <div className="absolute left-1/2 top-1/2 h-[38rem] w-[38rem] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/[0.06] blur-3xl" />
        </div>

        {/* z-10 — gentle scrim behind the top copy for AA legibility; the
            lower band stays clear so the pipeline reads. Token-based. */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 z-10 bg-[radial-gradient(ellipse_60%_38%_at_50%_24%,color-mix(in_srgb,var(--color-background)_58%,transparent)_0%,transparent_70%)]"
        />

        {/* z-50 — hero copy anchored to the TOP region (no card box). */}
        <div className="relative z-50 flex min-h-dvh flex-col items-center justify-start px-4 pb-24 pt-[6vh] text-center sm:pt-[10vh]">
          <div className="w-full max-w-2xl [&_h1]:[text-shadow:0_2px_24px_rgba(2,6,23,0.9)] [&_p]:[text-shadow:0_1px_14px_rgba(2,6,23,0.95)]">
            <span className="mb-6 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.05] px-3 py-1 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground backdrop-blur-sm">
              <Boxes className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
              Distributed task queue
            </span>

            <h1 className="text-balance text-5xl font-bold tracking-tight text-foreground sm:text-7xl">
              Task Queue
            </h1>

            <p className="mx-auto mt-5 max-w-lg text-balance text-lg text-muted-foreground sm:text-xl">
              See how work flows through a distributed queue — enqueue, lease, process, and recover
              from crashes, step by step.
            </p>

            <div className="mt-9 flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
              <Button asChild size="lg" className="cursor-pointer">
                <Link href="/playground">
                  Open Playground
                  <ArrowRight className="h-4 w-4" aria-hidden="true" />
                </Link>
              </Button>
              <Button asChild size="lg" variant="outline" className="cursor-pointer bg-card/50 backdrop-blur-sm">
                <Link href="/learn">
                  <BookOpen className="h-4 w-4" aria-hidden="true" />
                  Start Learning
                </Link>
              </Button>
            </div>
          </div>
        </div>

        {/* Scroll affordance — pinned to the very bottom, gentle bounce. */}
        <a
          href="#learn-more"
          aria-label="Scroll to learn more"
          className="absolute bottom-2 left-1/2 z-40 flex -translate-x-1/2 cursor-pointer flex-col items-center gap-1 text-muted-foreground transition-colors hover:text-foreground"
        >
          <span className="font-mono text-[0.6rem] uppercase tracking-[0.2em]">Scroll</span>
          <ChevronDown className="h-4 w-4 animate-bounce motion-reduce:animate-none" aria-hidden="true" />
        </a>
      </QueueTutorialProvider>
    </section>
  );
}
