"use client";

import Link from "next/link";
import { GraduationCap, Clock, CheckCircle2, Lock, ArrowRight } from "lucide-react";
import {
  getChaptersByPhase,
  phaseTone,
  type Chapter,
} from "@/lib/learn";
import { useReadChapters } from "@/lib/learnProgress";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

function ChapterCard({ chapter, read }: { chapter: Chapter; read: boolean }) {
  const tone = phaseTone[chapter.phaseGroup];
  const available = chapter.status === "available";

  const body = (
    <Card
      className={cn(
        "h-full gap-0 py-0 transition-colors",
        available
          ? "group-hover:border-primary/50 group-focus-visible:border-primary/50"
          : "opacity-60"
      )}
    >
      <CardHeader className="gap-0 px-5 pt-5">
        <div className="flex items-start gap-3">
          <span
            className={cn(
              "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg font-mono text-sm font-bold ring-1 ring-inset",
              tone.chip,
              tone.ink,
              tone.ring
            )}
            aria-hidden="true"
          >
            {chapter.order}
          </span>
          <div className="min-w-0 flex-1">
            <CardTitle className="flex items-start gap-2 text-base leading-snug">
              <span className="min-w-0 flex-1">{chapter.title}</span>
              {available && read && (
                <CheckCircle2
                  className="mt-0.5 h-4 w-4 shrink-0 text-primary"
                  aria-label="Read"
                />
              )}
            </CardTitle>
            <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
              {chapter.subtitle}
            </p>
          </div>
        </div>
      </CardHeader>

      <CardContent className="mt-4 flex items-center justify-between px-5 pb-5">
        <span className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
          <Clock className="h-3.5 w-3.5" aria-hidden="true" />
          {chapter.readingMinutes} min read
        </span>
        {available ? (
          <span className="inline-flex items-center gap-1.5 text-sm font-semibold text-primary">
            {read ? "Re-read" : "Start"}
            <ArrowRight
              className="h-4 w-4 transition-transform group-hover:translate-x-0.5"
              aria-hidden="true"
            />
          </span>
        ) : (
          <Badge variant="outline" className="gap-1 text-muted-foreground">
            <Lock className="h-3 w-3" aria-hidden="true" />
            Coming soon
          </Badge>
        )}
      </CardContent>
    </Card>
  );

  if (!available) {
    return <div aria-disabled="true">{body}</div>;
  }

  return (
    <Link
      href={`/learn/${chapter.slug}`}
      className="group block rounded-xl focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
    >
      {body}
    </Link>
  );
}

export default function LearnIndexPage() {
  const groups = getChaptersByPhase();
  const { isRead } = useReadChapters();

  return (
    <div className="mx-auto max-w-[1000px] px-4 py-10 space-y-12">
      <header>
        <div className="flex items-center gap-4">
          <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg border border-border bg-card text-primary">
            <GraduationCap className="h-6 w-6" aria-hidden="true" />
          </span>
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.12em] text-primary">
              Curriculum
            </p>
            <h1 className="mt-1 text-3xl font-bold tracking-tight text-foreground">
              Learn
            </h1>
          </div>
        </div>
        <p className="mt-4 max-w-2xl text-muted-foreground">
          A guided path from &ldquo;what is a task queue?&rdquo; to the
          distributed internals powering this playground.
        </p>
      </header>

      {groups.map(({ phase, chapters }) => {
        if (chapters.length === 0) return null;
        const tone = phaseTone[phase];
        return (
          <section key={phase} aria-labelledby={`phase-${phase}`}>
            <div className="mb-4 flex items-center gap-3">
              <Badge
                className={cn(
                  "rounded-md font-mono uppercase tracking-[0.1em] ring-1 ring-inset",
                  tone.chip,
                  tone.ink,
                  tone.ring
                )}
              >
                {phase}
              </Badge>
              <span
                id={`phase-${phase}`}
                className="font-mono text-xs text-muted-foreground"
              >
                {chapters.length} chapter{chapters.length === 1 ? "" : "s"}
              </span>
            </div>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {chapters.map((chapter) => (
                <ChapterCard
                  key={chapter.slug}
                  chapter={chapter}
                  read={isRead(chapter.slug)}
                />
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}
