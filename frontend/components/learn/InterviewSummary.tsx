import { Briefcase } from "lucide-react";

/** Collapsible "interview-ready" recap using native <details> (no JS needed). */
export default function InterviewSummary({ summary }: { summary: string }) {
  return (
    <details className="group mt-6 overflow-hidden rounded-xl border border-border border-l-4 border-l-primary bg-card">
      <summary className="flex cursor-pointer list-none items-center gap-2.5 px-5 py-4 [&::-webkit-details-marker]:hidden">
        <Briefcase className="h-5 w-5 text-primary" aria-hidden="true" />
        <span className="font-mono text-xs uppercase tracking-[0.12em] text-primary">
          Interview Summary
        </span>
        <span className="ml-auto text-xs font-medium text-muted-foreground group-open:hidden">
          Show
        </span>
        <span className="ml-auto hidden text-xs font-medium text-muted-foreground group-open:inline">
          Hide
        </span>
      </summary>
      <div className="border-t border-border px-5 py-4 text-base leading-7 text-foreground/85">
        {summary}
      </div>
    </details>
  );
}
