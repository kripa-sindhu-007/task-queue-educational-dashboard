import { Briefcase } from "lucide-react";

/** Collapsible "interview-ready" recap using native <details> (no JS needed). */
export default function InterviewSummary({ summary }: { summary: string }) {
  return (
    <details className="clay group mt-6 overflow-hidden rounded-2xl">
      <summary className="clay-btn flex cursor-pointer list-none items-center gap-2.5 bg-card px-5 py-3.5 font-display font-semibold text-foreground [&::-webkit-details-marker]:hidden">
        <Briefcase className="h-5 w-5 text-sky-ink" aria-hidden="true" />
        Interview Summary
        <span className="ml-auto text-xs font-medium text-muted-foreground group-open:hidden">
          Show
        </span>
        <span className="ml-auto hidden text-xs font-medium text-muted-foreground group-open:inline">
          Hide
        </span>
      </summary>
      <div className="px-5 pb-5 pt-1 text-[15px] leading-7 text-foreground/85">
        {summary}
      </div>
    </details>
  );
}
