"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Boxes, GraduationCap, Gamepad2, Code2 } from "lucide-react";
import { cn } from "@/lib/utils";

const REPO_URL =
  "https://github.com/kripa-sindhu-007/task-queue-educational-dashboard";

const links = [
  { href: "/playground", label: "Playground", icon: Gamepad2 },
  { href: "/learn", label: "Learn", icon: GraduationCap },
];

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export default function SiteNav() {
  const pathname = usePathname();

  return (
    <nav
      aria-label="Primary"
      className="sticky top-0 z-50 border-b border-white/[0.06] bg-background/70 backdrop-blur-xl"
    >
      {/* subtle top accent hairline */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent"
      />
      <div className="mx-auto flex h-14 max-w-[1400px] items-center gap-3 px-4 sm:gap-4">
        {/* Brand */}
        <Link
          href="/"
          aria-current={pathname === "/" ? "page" : undefined}
          className="group flex items-center gap-2.5 rounded-lg focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          aria-label="Task Queue — home"
        >
          <span className="relative flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-primary/25 bg-gradient-to-br from-primary/25 to-primary/[0.06] text-primary shadow-[0_0_18px_-6px_var(--color-primary)] transition-transform group-hover:scale-105">
            <Boxes className="h-[18px] w-[18px]" aria-hidden="true" />
          </span>
          <span className="hidden items-baseline gap-2 sm:flex">
            <span className="text-[15px] font-semibold tracking-tight text-foreground">
              Task Queue
            </span>
            <span className="rounded border border-border/70 px-1.5 py-px font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              demo
            </span>
          </span>
        </Link>

        {/* Segmented nav */}
        <div className="ml-auto flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-full border border-white/[0.06] bg-muted/30 p-1 backdrop-blur">
            {links.map(({ href, label, icon: Icon }) => {
              const active = isActive(pathname, href);
              return (
                <Link
                  key={href}
                  href={href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "flex cursor-pointer items-center gap-2 rounded-full px-3 py-1.5 text-sm font-medium transition-all sm:px-4",
                    "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                    active
                      ? "bg-primary text-primary-foreground shadow-[0_2px_10px_-2px_var(--color-primary)]"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  <Icon className="h-4 w-4" aria-hidden="true" />
                  <span>{label}</span>
                </Link>
              );
            })}
          </div>

          {/* Source link */}
          <a
            href={REPO_URL}
            target="_blank"
            rel="noreferrer noopener"
            aria-label="View source on GitHub"
            className="hidden h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-full border border-white/[0.06] bg-muted/30 text-muted-foreground transition-colors hover:border-primary/40 hover:text-foreground focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 sm:flex"
          >
            <Code2 className="h-4 w-4" aria-hidden="true" />
          </a>
        </div>
      </div>
    </nav>
  );
}
