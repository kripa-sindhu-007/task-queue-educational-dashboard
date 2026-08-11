"use client";

import Link from "next/link";
import { GraduationCap, Clock, CheckCircle2, Lock } from "lucide-react";
import {
  getChaptersByPhase,
  phaseTone,
  type Chapter,
} from "@/lib/learn";
import { useReadChapters } from "@/lib/learnProgress";
import { cn } from "@/lib/utils";

function ChapterCard({ chapter, read }: { chapter: Chapter; read: boolean }) {
  const tone = phaseTone[chapter.phaseGroup];
  const available = chapter.status === "available";

  const inner = (
    <>
      <div className="flex items-start gap-3">
        <span
          className={cn(
            "clay-chip flex h-10 w-10 shrink-0 items-center justify-center rounded-xl font-display text-base font-bold",
            tone.chip,
            tone.ink
          )}
          aria-hidden="true"
        >
          {chapter.order}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="font-display text-lg font-semibold leading-snug text-foreground">
              {chapter.title}
            </h3>
            {available && read && (
              <CheckCircle2
                className="h-4 w-4 shrink-0 text-mint-ink"
                aria-label="Read"
              />
            )}
          </div>
          <p className="mt-0.5 text-sm text-muted-foreground">
            {chapter.subtitle}
          </p>
        </div>
      </div>

      <div className="mt-4 flex items-center gap-3 text-xs font-semibold">
        <span className="inline-flex items-center gap-1 text-muted-foreground">
          <Clock className="h-3.5 w-3.5" aria-hidden="true" />
          {chapter.readingMinutes} min read
        </span>
        {available ? (
          <span className={cn("clay-chip rounded-lg px-2 py-0.5", tone.chip, tone.ink)}>
            {read ? "Read" : "Start"}
          </span>
        ) : (
          <span className="clay-chip inline-flex items-center gap-1 rounded-lg bg-muted px-2 py-0.5 text-muted-foreground">
            <Lock className="h-3 w-3" aria-hidden="true" />
            Coming soon
          </span>
        )}
      </div>
    </>
  );

  if (!available) {
    return (
      <div
        aria-disabled="true"
        className="clay rounded-2xl bg-card p-5 opacity-55"
      >
        {inner}
      </div>
    );
  }

  return (
    <Link
      href={`/learn/${chapter.slug}`}
      className="clay-btn block rounded-2xl bg-card p-5"
    >
      {inner}
    </Link>
  );
}

export default function LearnIndexPage() {
  const groups = getChaptersByPhase();
  const { isRead } = useReadChapters();

  return (
    <div className="mx-auto max-w-[1000px] px-4 py-8 space-y-8">
      <header className="flex items-start gap-4">
        <div className="clay-chip flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-grape to-primary text-white float-soft">
          <GraduationCap className="h-7 w-7" />
        </div>
        <div>
          <h1 className="font-display text-3xl font-bold tracking-tight text-foreground">
            Learn
          </h1>
          <p className="text-sm text-muted-foreground font-medium">
            A guided path from "what is a task queue?" to the distributed
            internals powering this playground.
          </p>
        </div>
      </header>

      {groups.map(({ phase, chapters }) => {
        if (chapters.length === 0) return null;
        const tone = phaseTone[phase];
        return (
          <section key={phase} aria-labelledby={`phase-${phase}`}>
            <div className="mb-3 flex items-center gap-2">
              <span
                className={cn(
                  "clay-chip rounded-lg px-3 py-1 font-display text-sm font-bold",
                  tone.chip,
                  tone.ink
                )}
              >
                {phase}
              </span>
              <span
                id={`phase-${phase}`}
                className="text-xs font-semibold text-muted-foreground"
              >
                {chapters.length} chapter{chapters.length === 1 ? "" : "s"}
              </span>
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
