"use client";

import { useState } from "react";
import { HelpCircle, ChevronDown, CheckCircle2 } from "lucide-react";
import { cn } from "@/lib/utils";
import type { RevisionQuestion } from "@/lib/learn";

/**
 * Self-check cards. Each question shows a "Show answer" chip; tapping it reveals
 * an ideal, candidate-quality model answer so the learner can check their own.
 * The whole row is the toggle (one accessible control); the chip reflects state.
 */
export default function RevisionQuestions({
  questions,
}: {
  questions: RevisionQuestion[];
}) {
  const [open, setOpen] = useState<Set<number>>(() => new Set());

  const toggle = (i: number) => {
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(i)) next.delete(i);
      else next.add(i);
      return next;
    });
  };

  return (
    <section aria-labelledby="revision-heading" className="mt-10">
      <h2
        id="revision-heading"
        className="font-display text-2xl font-bold tracking-tight text-foreground mb-1 flex items-center gap-2"
      >
        <HelpCircle className="h-6 w-6 text-grape-ink" aria-hidden="true" />
        Revision Questions
      </h2>
      <p className="text-sm text-muted-foreground mb-4">
        Answer each one in your head first, then reveal the model answer to check
        yourself.
      </p>

      <ol className="space-y-3">
        {questions.map((item, i) => {
          const isOpen = open.has(i);
          const answerId = `revision-answer-${i}`;
          return (
            <li key={i}>
              <button
                type="button"
                onClick={() => toggle(i)}
                aria-expanded={isOpen}
                aria-controls={answerId}
                className="clay-btn flex w-full items-center gap-3 bg-card px-4 py-3 text-left"
              >
                <span className="clay-chip flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-grape-soft font-display text-sm font-bold text-grape-ink">
                  {i + 1}
                </span>
                <span className="min-w-0 flex-1 text-[15px] font-semibold text-foreground">
                  {item.q}
                </span>
                <span
                  className={cn(
                    "clay-chip inline-flex shrink-0 items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-bold whitespace-nowrap transition-colors duration-200 motion-reduce:transition-none",
                    isOpen
                      ? "bg-mint-soft text-mint-ink"
                      : "bg-grape-soft text-grape-ink"
                  )}
                >
                  <ChevronDown
                    className={cn(
                      "h-3.5 w-3.5 transition-transform duration-200 motion-reduce:transition-none",
                      isOpen && "rotate-180"
                    )}
                    aria-hidden="true"
                  />
                  {isOpen ? "Hide answer" : "Show answer"}
                </span>
              </button>
              {isOpen && (
                <div
                  id={answerId}
                  className="clay-inset mt-2 rounded-xl px-4 py-3"
                >
                  <div className="mb-1.5 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-mint-ink">
                    <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                    Model answer
                  </div>
                  <p className="text-sm leading-6 text-foreground/80">
                    {item.a}
                  </p>
                </div>
              )}
            </li>
          );
        })}
      </ol>
    </section>
  );
}
