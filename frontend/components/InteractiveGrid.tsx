"use client";

import { useEffect, useRef } from "react";

/**
 * A fixed, behind-content dev-grid that is nearly invisible by default and
 * "lights up" in a soft radius that follows the cursor. Pure CSS mask driven by
 * two custom properties (--mx / --my) updated on pointer move via rAF.
 */
export default function InteractiveGrid() {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    // Skip the cursor effect on touch / coarse pointers (no hover) — the faint
    // base grid still shows.
    const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)");
    if (!finePointer.matches) {
      el.style.setProperty("--spot", "0px"); // no spotlight, base grid only
      return;
    }

    let raf = 0;
    let x = window.innerWidth / 2;
    let y = window.innerHeight / 3;

    const apply = () => {
      raf = 0;
      el.style.setProperty("--mx", `${x}px`);
      el.style.setProperty("--my", `${y}px`);
    };
    const onMove = (e: PointerEvent) => {
      x = e.clientX;
      y = e.clientY;
      if (!raf) raf = requestAnimationFrame(apply);
    };

    apply();
    window.addEventListener("pointermove", onMove, { passive: true });
    return () => {
      window.removeEventListener("pointermove", onMove);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);

  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="pointer-events-none fixed inset-0 -z-10"
      style={
        {
          "--mx": "50%",
          "--my": "33%",
          "--spot": "260px",
          backgroundImage:
            "linear-gradient(to right, color-mix(in srgb, var(--color-border) 75%, transparent) 1px, transparent 1px), linear-gradient(to bottom, color-mix(in srgb, var(--color-border) 75%, transparent) 1px, transparent 1px)",
          backgroundSize: "46px 46px",
          // Bright at the cursor, fading to a faint ~5% floor everywhere else.
          WebkitMaskImage:
            "radial-gradient(var(--spot) circle at var(--mx) var(--my), #000 0%, rgba(0,0,0,0.85) 28%, rgba(0,0,0,0.05) 72%)",
          maskImage:
            "radial-gradient(var(--spot) circle at var(--mx) var(--my), #000 0%, rgba(0,0,0,0.85) 28%, rgba(0,0,0,0.05) 72%)",
        } as React.CSSProperties
      }
    />
  );
}
