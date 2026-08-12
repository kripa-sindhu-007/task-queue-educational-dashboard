import { Code2 } from "lucide-react";

const REPO_URL =
  "https://github.com/kripa-sindhu-007/task-queue-educational-dashboard";

export default function LandingFooter() {
  return (
    <footer className="border-t border-border/60">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 px-4 py-8 sm:flex-row">
        <p className="font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
          Go · Redis · Next.js
        </p>
        <a
          href={REPO_URL}
          target="_blank"
          rel="noreferrer noopener"
          className="inline-flex cursor-pointer items-center gap-2 rounded-md text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          <Code2 className="h-4 w-4" aria-hidden="true" />
          View source
        </a>
      </div>
    </footer>
  );
}
