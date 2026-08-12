"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import { Send, Database, Cpu, CircleCheckBig, Skull, RotateCcw } from "lucide-react";

/* -------------------------------------------------------------------------- *
 * QueueTutorial — a looping, self-explaining 2D diagram of the task-queue
 * lifecycle. It is composed of three pieces so the hero can layer them:
 *   <QueueTutorialProvider>  owns the single timeline (state) ─┐
 *     <QueueBackground/>      full-bleed animated diagram (z-0)│ shared frame
 *     <QueueStepper/>         synced caption + 5-step stepper  ─┘
 * One keyframe timeline drives BOTH the tokens and the caption/stepper, so the
 * teaching layer is always in sync with what's on screen.
 * -------------------------------------------------------------------------- */

type StateName =
  | "queued"
  | "leased"
  | "running"
  | "retrying"
  | "succeeded"
  | "failed"
  | "dead";

type WorkerState = "idle" | "leased" | "running" | "dead";

type Anchor = "producer" | "queue" | "workerA" | "workerB" | "workerC" | "done" | "dead";
type StageKey = "producer" | "queue" | "workers";

type TokenDef = {
  at: Anchor;
  slot?: number;
  st: StateName;
  lease?: number;
  expiring?: boolean;
  hidden?: boolean;
  jump?: boolean;
};

type Frame = {
  step: number;
  note: string;
  reaper?: boolean;
  workers: Partial<Record<"A" | "B" | "C", WorkerState>>;
  tokens: Record<string, TokenDef>;
  dur: number;
};

// Per-state "task packet" styling: a gradient body, a colored hairline ring,
// and a soft glow in the state hue. Class strings are literal so the Tailwind
// JIT picks them up.
const STATE_TOKEN: Record<StateName, { grad: string; ring: string; glow: string }> = {
  queued: {
    grad: "from-state-queued to-state-queued/55",
    ring: "ring-state-queued/70",
    glow: "shadow-[0_0_14px_-3px_var(--color-state-queued)]",
  },
  leased: {
    grad: "from-state-leased to-state-leased/55",
    ring: "ring-state-leased/70",
    glow: "shadow-[0_0_14px_-3px_var(--color-state-leased)]",
  },
  running: {
    grad: "from-state-running to-state-running/55",
    ring: "ring-state-running/70",
    glow: "shadow-[0_0_16px_-2px_var(--color-state-running)]",
  },
  retrying: {
    grad: "from-state-retrying to-state-retrying/55",
    ring: "ring-state-retrying/70",
    glow: "shadow-[0_0_14px_-3px_var(--color-state-retrying)]",
  },
  succeeded: {
    grad: "from-state-succeeded to-state-succeeded/55",
    ring: "ring-state-succeeded/70",
    glow: "shadow-[0_0_18px_-2px_var(--color-state-succeeded)]",
  },
  failed: {
    grad: "from-state-failed to-state-failed/55",
    ring: "ring-state-failed/70",
    glow: "shadow-[0_0_14px_-3px_var(--color-state-failed)]",
  },
  dead: {
    grad: "from-state-dead to-state-dead/55",
    ring: "ring-state-dead/60",
    glow: "shadow-[0_0_10px_-4px_var(--color-state-dead)]",
  },
};

const WORKER_TONE: Record<WorkerState, string> = {
  idle: "border-border/70 bg-card/70 text-muted-foreground",
  leased: "border-state-leased/60 bg-state-leased/10 text-foreground",
  running: "border-state-running/60 bg-state-running/10 text-foreground",
  dead: "border-state-dead/40 bg-card/40 text-muted-foreground opacity-60",
};

const WORKER_DOT: Record<WorkerState, string> = {
  idle: "bg-muted-foreground/50",
  leased: "bg-state-leased",
  running: "bg-state-running",
  dead: "bg-state-dead",
};

export const STEPS = [
  { n: 1, title: "Submit", desc: "A client enqueues a task instead of running it inline." },
  { n: 2, title: "Queue", desc: "It waits in a Redis-backed queue, ordered by priority." },
  { n: 3, title: "Lease", desc: "A worker leases the task for a bounded time window." },
  { n: 4, title: "Process & ack", desc: "The worker runs it and acks — at-least-once delivery." },
  {
    n: 5,
    title: "Recover",
    desc: "If a worker crashes, its lease expires and a reaper requeues the task — no work lost.",
  },
] as const;

const ACTIVE_STAGE: Record<number, StageKey> = {
  0: "producer",
  1: "queue",
  2: "workers",
  3: "workers",
  4: "workers",
};

const Q: StateName = "queued";
const L: StateName = "leased";
const R: StateName = "running";
const S: StateName = "succeeded";
const F: StateName = "failed";
const RETRY: StateName = "retrying";

const TIMELINE: Frame[] = [
  {
    step: 0,
    note: "client creates a task",
    workers: {},
    dur: 1500,
    tokens: {
      q1: { at: "queue", slot: 0, st: Q },
      q2: { at: "queue", slot: 1, st: Q },
      a: { at: "producer", st: Q, jump: true },
    },
  },
  {
    step: 1,
    note: "enqueued — waiting by priority",
    workers: {},
    dur: 1700,
    tokens: {
      q1: { at: "queue", slot: 0, st: Q },
      q2: { at: "queue", slot: 1, st: Q },
      a: { at: "queue", slot: 2, st: Q },
    },
  },
  {
    step: 2,
    note: "a worker leases the front task",
    workers: { A: "leased" },
    dur: 1500,
    tokens: {
      q1: { at: "workerA", st: L, lease: 1 },
      q2: { at: "queue", slot: 0, st: Q },
      a: { at: "queue", slot: 1, st: Q },
    },
  },
  {
    step: 3,
    note: "workers run the tasks",
    workers: { A: "running", B: "running" },
    dur: 1600,
    tokens: {
      q1: { at: "workerA", st: R },
      q2: { at: "workerB", st: R },
      a: { at: "queue", slot: 0, st: Q },
    },
  },
  {
    step: 3,
    note: "ack ✓ done · nack ✗ → dead-letter",
    workers: {},
    dur: 1700,
    tokens: {
      q1: { at: "done", st: S },
      q2: { at: "dead", st: F },
      a: { at: "queue", slot: 0, st: Q },
    },
  },
  {
    step: 4,
    note: "a worker leases the task",
    workers: { C: "leased" },
    dur: 1400,
    tokens: {
      q1: { at: "done", st: S, hidden: true },
      q2: { at: "dead", st: F, hidden: true },
      a: { at: "workerC", st: L, lease: 1 },
    },
  },
  {
    step: 4,
    note: "processing under a lease timer",
    workers: { C: "running" },
    dur: 1500,
    tokens: {
      q1: { at: "done", st: S, hidden: true },
      q2: { at: "dead", st: F, hidden: true },
      a: { at: "workerC", st: R, lease: 0.35 },
    },
  },
  {
    step: 4,
    note: "worker crashed — lease expired",
    reaper: true,
    workers: { C: "dead" },
    dur: 1700,
    tokens: {
      q1: { at: "done", st: S, hidden: true },
      q2: { at: "dead", st: F, hidden: true },
      a: { at: "workerC", st: L, lease: 0.06, expiring: true },
    },
  },
  {
    step: 4,
    note: "reaped → requeued — no work lost",
    reaper: true,
    workers: { C: "dead" },
    dur: 1800,
    tokens: {
      q1: { at: "done", st: S, hidden: true },
      q2: { at: "dead", st: F, hidden: true },
      a: { at: "queue", slot: 0, st: RETRY },
    },
  },
  {
    step: 4,
    note: "recovered — ready to retry",
    workers: {},
    dur: 1500,
    tokens: {
      q1: { at: "done", st: S, hidden: true },
      q2: { at: "dead", st: F, hidden: true },
      a: { at: "queue", slot: 0, st: Q },
    },
  },
];

const STATIC_FRAME: Frame = {
  step: -1,
  note: "",
  workers: { A: "leased", B: "running", C: "dead" },
  dur: 0,
  tokens: {
    sq0: { at: "queue", slot: 0, st: Q },
    sq1: { at: "queue", slot: 1, st: Q },
    sq2: { at: "queue", slot: 2, st: RETRY },
    sl: { at: "workerA", st: L, lease: 0.6 },
    sr: { at: "workerB", st: R },
    ss: { at: "done", st: S },
    sf: { at: "dead", st: F },
  },
};

/* -------------------------------- context -------------------------------- */

const QueueCtx = createContext<{ frame: Frame; reduce: boolean }>({
  frame: STATIC_FRAME,
  reduce: true,
});
const useQueue = () => useContext(QueueCtx);

export function QueueTutorialProvider({ children }: { children: React.ReactNode }) {
  const reduce = useReducedMotion() ?? false;
  const [frameIdx, setFrameIdx] = useState(0);

  useEffect(() => {
    if (reduce) return;
    let id: ReturnType<typeof setTimeout>;
    let i = 0;
    const tick = () => {
      setFrameIdx(i);
      const dur = TIMELINE[i].dur;
      i = (i + 1) % TIMELINE.length;
      id = setTimeout(tick, dur);
    };
    tick();
    return () => clearTimeout(id);
  }, [reduce]);

  const frame = reduce ? STATIC_FRAME : TIMELINE[frameIdx];
  return <QueueCtx.Provider value={{ frame, reduce }}>{children}</QueueCtx.Provider>;
}

/* -------------------------------- helpers -------------------------------- */

type Point = { x: number; y: number };

// Tokens anchor to REAL measured dock elements (queue slot markers, worker
// docks, producer/outcome docks) so they seat cleanly inside every box across
// breakpoints — no eyeballed offsets.
function tokenPos(
  anchors: Partial<Record<Anchor, Point>>,
  slots: Point[],
  def: TokenDef,
): { x: number; y: number; opacity: number } {
  if (def.at === "queue" && def.slot != null && slots[def.slot]) {
    const s = slots[def.slot];
    return { x: s.x, y: s.y, opacity: def.hidden ? 0 : 1 };
  }
  const base = anchors[def.at];
  if (!base) return { x: 0, y: 0, opacity: 0 };
  return { x: base.x, y: base.y, opacity: def.hidden ? 0 : 1 };
}

function TokenDot({
  def,
  pos,
  reduce,
}: {
  def: TokenDef;
  pos: { x: number; y: number; opacity: number };
  reduce: boolean;
}) {
  const tok = STATE_TOKEN[def.st];
  return (
    <motion.div
      className="absolute left-0 top-0"
      initial={false}
      animate={{ x: pos.x, y: pos.y, opacity: pos.opacity }}
      transition={
        def.jump || reduce
          ? { duration: 0 }
          : { type: "tween", ease: "easeInOut", duration: 0.5 }
      }
    >
      <div className="-translate-x-1/2 -translate-y-1/2">
        {/* Task "packet": gradient body + gloss + payload lines. Sized to seat
            inside the 26px docks with breathing room. */}
        <div
          className={`relative h-[22px] w-[22px] rounded-[7px] bg-gradient-to-b ${tok.grad} ring-1 ${tok.ring} ${tok.glow}`}
        >
          {/* top gloss highlight */}
          <span
            aria-hidden="true"
            className="pointer-events-none absolute inset-x-[3px] top-[2px] h-[6px] rounded-full bg-white/35 blur-[1.5px]"
          />
          {/* payload content lines */}
          <span
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-[2px]"
          >
            <span className="h-[2px] w-[11px] rounded-full bg-black/45" />
            <span className="h-[2px] w-[7px] rounded-full bg-black/30" />
          </span>
          {def.st === "running" && (
            <span className="absolute -inset-[3px] rounded-[9px] ring-2 ring-state-running/70 animate-ping motion-reduce:animate-none" />
          )}
          {def.lease != null && (
            <span className="absolute -bottom-2 left-1/2 h-[3px] w-[22px] -translate-x-1/2 overflow-hidden rounded-full bg-black/50">
              <motion.span
                className={`block h-full rounded-full ${def.expiring ? "bg-state-failed" : "bg-state-leased"}`}
                style={{ transformOrigin: "left" }}
                initial={false}
                animate={{ scaleX: def.lease }}
                transition={reduce ? { duration: 0 } : { type: "tween", ease: "easeInOut", duration: 0.6 }}
              />
            </span>
          )}
        </div>
      </div>
    </motion.div>
  );
}

// Each stage is a little titled card: a mono/uppercase/muted label header on
// top, the icon + dashed dock (where the token seats) inside the same box.
const CARD =
  "flex flex-col gap-1 rounded-lg border bg-card/60 px-2 py-1.5 shadow-sm backdrop-blur-sm transition-all duration-300";
const DOCK =
  "h-[26px] w-[26px] shrink-0 rounded-[7px] border border-dashed border-border/50 bg-background/30";
const BOX_LABEL = "font-mono text-[9px] uppercase tracking-[0.12em] text-muted-foreground";

function WorkerNode({
  label,
  state,
  dockRef,
}: {
  label: string;
  state: WorkerState;
  dockRef: (el: HTMLElement | null) => void;
}) {
  return (
    <div
      className={`flex items-center gap-1.5 rounded-md border px-1.5 py-1 transition-colors duration-300 ${WORKER_TONE[state]}`}
    >
      {/* Dedicated task dock — the token seats HERE, beside the label. */}
      <span ref={dockRef} className={DOCK} />
      <Cpu className="size-3 shrink-0" />
      <span className="whitespace-nowrap font-mono text-[10px] leading-none">{label}</span>
      {state === "dead" ? (
        <Skull className="size-3 shrink-0 text-state-dead" />
      ) : (
        <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${WORKER_DOT[state]}`} />
      )}
    </div>
  );
}

/* ----------------------- integrated synced caption ----------------------- */

// Lives INSIDE the animation band (above the pipeline row) so the explanation
// and the diagram read as one view. Driven by the same `frame.step`.

// Reduced-motion fallback: a static legend of all five phases (rendered in
// flow above the pipeline). No moving caption.
function StaticLegend() {
  return (
    <div className="mb-3 flex flex-col items-center gap-1 px-2 text-center [&_*]:[text-shadow:0_1px_10px_rgba(2,6,23,0.85)]">
      <div className="mb-0.5 flex items-center gap-1.5" aria-hidden="true">
        {STEPS.map((s) => (
          <span key={s.n} className="h-1.5 w-1.5 rounded-full bg-muted-foreground/50" />
        ))}
      </div>
      <ol className="flex flex-col items-center gap-0.5">
        {STEPS.map((s) => (
          <li key={s.n} className="text-[10px] leading-snug text-muted-foreground">
            <span className="font-mono uppercase tracking-[0.1em] text-foreground">
              <span className="text-primary">{s.n}</span> {s.title}
            </span>
            <span className="hidden sm:inline"> — {s.desc}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}

// Contextual caption: floats at the active step's location in the pipeline and
// glides to the next stage as `step` advances (x/y driven by measured anchors).
function FloatingCaption({ x, y, step }: { x: number; y: number; step: number }) {
  return (
    <motion.div
      className="pointer-events-none absolute left-0 top-0 z-30"
      initial={false}
      animate={{ x, y }}
      transition={{ type: "spring", stiffness: 210, damping: 26 }}
    >
      <div className="-translate-x-1/2 -translate-y-1/2">
        <div className="relative flex max-w-[104px] flex-col items-center gap-1 rounded-lg border border-white/10 bg-card/85 px-2.5 py-1.5 text-center shadow-lg shadow-black/40 backdrop-blur-md md:max-w-[224px]">
          {/* overall progress — active tick in the accent */}
          <div className="flex items-center gap-1" aria-hidden="true">
            {STEPS.map((s, i) => (
              <span
                key={s.n}
                className={`h-1 rounded-full transition-all duration-300 ${
                  i === step ? "w-3.5 bg-primary" : "w-1 bg-muted-foreground/40"
                }`}
              />
            ))}
          </div>
          <p className="font-mono text-[10px] uppercase leading-none tracking-[0.1em] text-foreground md:text-[11px]">
            <span className="text-primary">{STEPS[step].n}</span> · {STEPS[step].title}
          </p>
          <p className="text-[10px] leading-snug text-muted-foreground">{STEPS[step].desc}</p>
          {/* pointer — down on desktop (above the row), right on mobile (beside the box) */}
          <span className="absolute -bottom-1 left-1/2 hidden h-2 w-2 -translate-x-1/2 rotate-45 border-b border-r border-white/10 bg-card/85 md:block" />
          <span className="absolute -right-1 top-1/2 block h-2 w-2 -translate-y-1/2 rotate-45 border-b border-r border-white/10 bg-card/85 md:hidden" />
        </div>
      </div>
    </motion.div>
  );
}

/* --------------------------- background diagram --------------------------- */

export function QueueBackground() {
  const { frame, reduce } = useQueue();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const nodeRefs = useRef<Partial<Record<Anchor | "workersBox", HTMLElement | null>>>({});
  const slotRefs = useRef<(HTMLElement | null)[]>([]);

  const [orientation, setOrientation] = useState<"horizontal" | "vertical">("horizontal");
  const [anchors, setAnchors] = useState<Partial<Record<Anchor, Point>>>({});
  const [slots, setSlots] = useState<Point[]>([]);
  const [size, setSize] = useState<{ w: number; h: number }>({ w: 0, h: 0 });

  const activeStage = reduce ? null : ACTIVE_STAGE[frame.step];

  useEffect(() => {
    const mq = window.matchMedia("(min-width: 768px)");
    const apply = () => setOrientation(mq.matches ? "horizontal" : "vertical");
    apply();
    mq.addEventListener("change", apply);
    return () => mq.removeEventListener("change", apply);
  }, []);

  const measure = useCallback(() => {
    const c = containerRef.current;
    if (!c) return;
    const cr = c.getBoundingClientRect();
    const center = (el: HTMLElement): Point => {
      const r = el.getBoundingClientRect();
      return { x: r.left - cr.left + r.width / 2, y: r.top - cr.top + r.height / 2 };
    };
    const next: Partial<Record<Anchor, Point>> = {};
    (Object.keys(nodeRefs.current) as (Anchor | "workersBox")[]).forEach((k) => {
      const el = nodeRefs.current[k];
      if (el && k !== "workersBox") next[k as Anchor] = center(el);
    });
    const slotPts: Point[] = [];
    slotRefs.current.forEach((el, i) => {
      if (el) slotPts[i] = center(el);
    });
    setAnchors(next);
    setSlots(slotPts);
    setSize({ w: cr.width, h: cr.height });
  }, []);

  useEffect(() => {
    measure();
    const c = containerRef.current;
    if (!c) return;
    const ro = new ResizeObserver(() => measure());
    ro.observe(c);
    window.addEventListener("resize", measure);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [measure, orientation]);

  const setRef = (k: Anchor | "workersBox") => (el: HTMLElement | null) => {
    nodeRefs.current[k] = el;
  };

  const workersBox = (() => {
    const el = nodeRefs.current.workersBox;
    const c = containerRef.current;
    if (!el || !c) return undefined;
    const cr = c.getBoundingClientRect();
    const r = el.getBoundingClientRect();
    return { x: r.left - cr.left + r.width / 2, y: r.top - cr.top + r.height / 2 };
  })();

  const line = (a?: Point, b?: Point) =>
    a && b ? { x1: a.x, y1: a.y, x2: b.x, y2: b.y } : null;

  const connectors = [
    line(anchors.producer, anchors.queue),
    line(anchors.queue, workersBox),
    line(workersBox, anchors.done),
    line(workersBox, anchors.dead),
  ].filter(Boolean) as { x1: number; y1: number; x2: number; y2: number }[];

  const recyclePath =
    workersBox && anchors.queue
      ? `M ${workersBox.x} ${workersBox.y} C ${(workersBox.x + anchors.queue.x) / 2} ${workersBox.y + 70}, ${(workersBox.x + anchors.queue.x) / 2} ${anchors.queue.y + 70}, ${anchors.queue.x} ${anchors.queue.y}`
      : null;

  const glow = (on: boolean) =>
    on ? "border-primary/60 shadow-lg shadow-primary/30" : "border-border/70";

  // Contextual caption anchor per step → glides along the pipeline.
  //   0 Submit → producer · 1 Queue → queue · 2 Lease → gap(queue,workers)
  //   3 Process → workers  · 4 Recover → gap(workers,queue) (reaper loop-back)
  const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));
  const mid = (a?: Point, b?: Point) =>
    a && b ? { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 } : undefined;
  const stepAnchors: (Point | undefined)[] = [
    anchors.producer,
    anchors.queue,
    mid(anchors.queue, workersBox),
    workersBox,
    mid(workersBox, anchors.queue),
  ];
  const rawAnchor = reduce ? undefined : stepAnchors[frame.step];
  const cap = (() => {
    if (!rawAnchor || size.w === 0) return null;
    const pad = 6;
    if (orientation === "horizontal") {
      const halfW = 114;
      const halfH = 34;
      return {
        x: clamp(rawAnchor.x, halfW + pad, size.w - halfW - pad),
        y: clamp(size.h * 0.2, halfH + pad, size.h - halfH - pad),
      };
    }
    // Mobile: sit in the clear left lane, aligned to the active stage's row.
    const halfW = 52;
    const halfH = 42;
    return {
      x: clamp(size.w * 0.15, halfW + pad, size.w - halfW - pad),
      y: clamp(rawAnchor.y, halfH + pad, size.h - halfH - pad),
    };
  })();

  return (
    <div className="relative h-full w-full opacity-70 md:opacity-100">
      <p className="sr-only">
        An animated diagram of a distributed task queue. A producer enqueues tasks into a
        Redis-backed queue. Workers lease tasks for a bounded time window and process them.
        Successful tasks are acknowledged and moved to a done store, while failed tasks are routed
        to a dead-letter queue. If a worker crashes, its lease expires and a reaper requeues the
        task so no work is lost.
      </p>

      {/* Pipeline lives in the LOWER band so it never sits behind the top copy;
          clearance is left below for the bottom stepper strip. */}
      <div
        ref={containerRef}
        aria-hidden="true"
        className="absolute inset-x-4 top-[47%] bottom-[6%] md:inset-x-12 md:top-[48%] md:bottom-[9%]"
      >
        <svg
          className="absolute inset-0 h-full w-full text-primary"
          width={size.w}
          height={size.h}
          fill="none"
        >
          <defs>
            <marker id="qt-arrow" markerWidth="8" markerHeight="8" refX="6" refY="4" orient="auto">
              <path d="M1,1 L6,4 L1,7" stroke="currentColor" strokeWidth="1.2" fill="none" />
            </marker>
          </defs>
          {connectors.map((l, i) => (
            <line
              key={i}
              x1={l.x1}
              y1={l.y1}
              x2={l.x2}
              y2={l.y2}
              stroke="currentColor"
              strokeOpacity={0.35}
              strokeWidth={1.5}
              strokeLinecap="round"
              markerEnd="url(#qt-arrow)"
            />
          ))}
          {recyclePath && (
            <path
              d={recyclePath}
              className={frame.reaper ? "text-state-retrying" : "text-muted-foreground"}
              stroke="currentColor"
              strokeOpacity={frame.reaper ? 0.9 : 0.18}
              strokeWidth={1.5}
              strokeLinecap="round"
              strokeDasharray="4 4"
              markerEnd="url(#qt-arrow)"
            />
          )}
        </svg>

        <div className="relative z-10 flex h-full flex-col">
          {/* Reduced motion: a static legend stands in for the moving caption. */}
          {reduce && <StaticLegend />}

          <div className="flex flex-1 flex-col items-center justify-between gap-2 md:flex-row md:items-center">
          {/* Producer — far left */}
          <div className={`${CARD} ${glow(activeStage === "producer")}`}>
            <span className={BOX_LABEL}>producer</span>
            <div className="flex items-center gap-1.5">
              <Send className="size-4 shrink-0 text-primary" />
              <span ref={setRef("producer")} className={DOCK} />
            </div>
          </div>

          {/* Queue lane — left of center */}
          <div
            ref={setRef("queue")}
            className={`${CARD} w-[112px] md:w-[96px] ${glow(activeStage === "queue")}`}
          >
            <span className={`${BOX_LABEL} flex items-center gap-1`}>
              <Database className="size-3 text-state-queued" />
              queue · redis
            </span>
            {/* Real slot markers — tokens anchor to these measured centers. */}
            <div className="flex flex-row items-center justify-center gap-2 md:flex-col">
              {[0, 1, 2].map((s) => (
                <span
                  key={s}
                  ref={(el) => {
                    slotRefs.current[s] = el;
                  }}
                  className="h-[26px] w-[26px] rounded-[7px] border border-dashed border-border/50 bg-background/30"
                />
              ))}
            </div>
          </div>

          {/* Workers cluster — right of center */}
          <div className="relative flex flex-col items-center">
            <div
              ref={setRef("workersBox")}
              className={`${CARD} ${activeStage === "workers" ? "border-primary/50 shadow-lg shadow-primary/20" : ""}`}
            >
              <span className={BOX_LABEL}>workers</span>
              <div className="flex flex-col gap-1">
                <WorkerNode label="worker-1" state={frame.workers.A ?? "idle"} dockRef={setRef("workerA")} />
                <WorkerNode label="worker-2" state={frame.workers.B ?? "idle"} dockRef={setRef("workerB")} />
                <WorkerNode label="worker-3" state={frame.workers.C ?? "idle"} dockRef={setRef("workerC")} />
              </div>
            </div>
            <AnimatePresence>
              {frame.reaper && (
                <motion.span
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  className="absolute -bottom-6 flex items-center gap-1 rounded-full border border-state-retrying/50 bg-state-retrying/10 px-2 py-0.5 font-mono text-[9px] uppercase tracking-[0.12em] text-state-retrying"
                >
                  <RotateCcw className="h-3 w-3" />
                  reaper
                </motion.span>
              )}
            </AnimatePresence>
          </div>

          {/* Outcomes — far right */}
          <div className="flex flex-row gap-2 md:flex-col md:gap-3">
            <div className={CARD}>
              <span className={BOX_LABEL}>done</span>
              <div className="flex items-center gap-1.5">
                <CircleCheckBig className="size-4 shrink-0 text-state-succeeded" />
                <span ref={setRef("done")} className={DOCK} />
              </div>
            </div>
            <div className={CARD}>
              <span className={BOX_LABEL}>dead-letter</span>
              <div className="flex items-center gap-1.5">
                <Skull className="size-4 shrink-0 text-state-failed" />
                <span ref={setRef("dead")} className={DOCK} />
              </div>
            </div>
          </div>
          </div>
        </div>

        {/* Token overlay. */}
        <div className="pointer-events-none absolute inset-0 z-20">
          {anchors.producer &&
            Object.entries(frame.tokens).map(([id, def]) => (
              <TokenDot key={id} def={def} pos={tokenPos(anchors, slots, def)} reduce={reduce} />
            ))}
        </div>

        {/* Contextual, moving explanation caption (motion only). */}
        {cap && <FloatingCaption x={cap.x} y={cap.y} step={frame.step} />}
      </div>
    </div>
  );
}
