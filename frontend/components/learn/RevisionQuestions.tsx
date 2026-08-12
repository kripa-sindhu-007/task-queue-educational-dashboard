"use client";

import { useState } from "react";
import { HelpCircle, ChevronDown, CheckCircle2 } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { RevisionQuestion } from "@/lib/learn";

/**
 * Self-check cards. Each question shows a "Show answer" button; tapping it
 * reveals an ideal, candidate-quality model answer so the learner can check
 * their own. The button is the single accessible control per question.
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
        className="mb-1 flex items-center gap-2 text-2xl font-bold tracking-tight text-foreground"
      >
        <HelpCircle className="h-6 w-6 text-primary" aria-hidden="true" />
        Revision Questions
      </h2>
      <p className="mb-5 text-sm text-muted-foreground">
        Answer each one in your head first, then reveal the model answer to check
        yourself.
      </p>

      <ol className="space-y-3">
        {questions.map((item, i) => {
          const isOpen = open.has(i);
          const answerId = `revision-answer-${i}`;
          return (
            <li key={i}>
              <Card className="gap-0 overflow-hidden py-0">
                <div className="flex items-start gap-3 p-4">
                  <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted font-mono text-sm font-bold text-foreground">
                    {i + 1}
                  </span>
                  <p className="min-w-0 flex-1 pt-0.5 text-[15px] font-medium text-foreground">
                    {item.q}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => toggle(i)}
                    aria-expanded={isOpen}
                    aria-controls={answerId}
                    className="shrink-0"
                  >
                    <ChevronDown
                      className={cn(
                        "h-3.5 w-3.5 transition-transform duration-200 motion-reduce:transition-none",
                        isOpen && "rotate-180"
                      )}
                      aria-hidden="true"
                    />
                    {isOpen ? "Hide answer" : "Show answer"}
                  </Button>
                </div>
                {isOpen && (
                  <div
                    id={answerId}
                    className="border-t border-border border-l-4 border-l-primary bg-muted/40 px-4 py-3"
                  >
                    <div className="mb-1.5 flex items-center gap-1.5 font-mono text-xs uppercase tracking-[0.1em] text-primary">
                      <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                      Model answer
                    </div>
                    <p className="text-sm leading-6 text-foreground/85">
                      {item.a}
                    </p>
                  </div>
                )}
              </Card>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
