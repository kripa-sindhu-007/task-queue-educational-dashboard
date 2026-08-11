"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Boxes, GraduationCap, Gamepad2 } from "lucide-react";
import { cn } from "@/lib/utils";

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
      className="sticky top-0 z-50 border-b border-border/60 bg-background/80 backdrop-blur-md"
    >
      <div className="mx-auto flex max-w-[1400px] items-center gap-3 px-4 py-3 sm:gap-4">
        <Link
          href="/"
          aria-current={pathname === "/" ? "page" : undefined}
          className="flex items-center gap-2.5 font-semibold text-foreground transition-opacity hover:opacity-80"
          aria-label="Task Queue — home"
        >
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border bg-card text-primary">
            <Boxes className="h-5 w-5" aria-hidden="true" />
          </span>
          <span className="hidden text-base sm:inline">Task Queue</span>
        </Link>

        <div className="ml-auto flex items-center gap-1.5">
          {links.map(({ href, label, icon: Icon }) => {
            const active = isActive(pathname, href);
            return (
              <Link
                key={href}
                href={href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "flex cursor-pointer items-center gap-2 rounded-md px-3.5 py-2 text-sm font-medium transition-colors sm:px-4",
                  "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                  active
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-secondary hover:text-foreground"
                )}
              >
                <Icon className="h-4 w-4" aria-hidden="true" />
                <span>{label}</span>
              </Link>
            );
          })}
        </div>
      </div>
    </nav>
  );
}
