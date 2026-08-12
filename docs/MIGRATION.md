# Plan — Migrate manual Tailwind → shadcn/ui component library

**Project:** task-queue (`task-queue-educational-dashboard`)
**Scope:** `frontend/` only (Next.js 15 App Router, React 19, TS, Tailwind CSS v4 CSS-first)
**Track:** full
**Status:** awaiting ★ PLAN approval
**Date:** 2026-07-30

---

## 0. Executive summary

`shadcn init` has been run (`components.json` is valid: `new-york`, rsc, tsx, `neutral`,
cssVariables, lucide). Nothing else about the project is actually shadcn yet:

| Claim | Verified state |
|---|---|
| shadcn components installed | NO — `components/ui/*` are 7 hand-rolled lookalikes |
| Radix / Base UI primitives present | NO — zero `@radix-ui/*` or `@base-ui/*` in `package.json` |
| globals.css follows the v4 token contract | NO — raw hex inside `@theme`, no `:root`/`.dark`, no `@theme inline` |
| dark mode | NO — no `.dark` block, no `@custom-variant dark`, zero `dark:` classes anywhere |
| animation utilities the fake Dialog references | NO — `tw-animate-css` absent, so `animate-in fade-in zoom-in-95` are dead classes |

So this is not a "swap in shadcn components" job. It is three jobs stacked:

1. **Rebuild the theme layer** onto shadcn's Tailwind v4 contract without losing claymorphism.
2. **Replace 7 counterfeit primitives** with real ones and re-apply clay styling to each.
3. **Adopt ~8 genuinely new components** to delete hand-rolled state machines and a11y defects.

The largest risk is not visual — it is that **TypeScript cannot catch the most likely failure
mode.** See 7.1.

---

## A. The central design decision (★ approve one)

The app's identity *is* its visual language: claymorphism + an 8-hue candy palette +
Fredoka/Nunito. That is the product, not decoration — it is a playful educational dashboard.

### Option A1 — shadcn as component layer, claymorphism as theme layer — RECOMMENDED

Take shadcn for behaviour, accessibility, and composition (Radix/Base UI primitives, `asChild`,
portals, focus traps, ARIA roles). Express clay entirely through tokens + cva variants so every
generated component inherits it. `--radius: 1.25rem` already feeds shadcn's derived
`--radius-sm/md/lg/xl` scale, so stock `rounded-md`/`rounded-lg` come out chunky-clay for free.
Cost: each regenerated primitive needs its clay variants re-applied once (that work is M2).
Benefit: identity preserved, a11y defects fixed, and future `shadcn add` calls land pre-themed.

### Option A2 — adopt the stock new-york neutral look

Delete clay, accept shadcn defaults. Fastest, zero re-styling. **Destroys the app's reason to
exist** — a grey neutral dashboard teaching distributed systems is indistinguishable from a
thousand admin templates, and the candy palette encodes queue-stage semantics (`phaseTone`,
`toneClasses`, `eventVariantMap`) that would have to be rebuilt anyway. Not recommended.

### Option A3 — hybrid: stock shadcn for "chrome", clay for hero surfaces

Stock styling inside dialogs/tables/tooltips, clay on cards and chips. Less re-styling up front.
But it puts two visual languages in one viewport (a neutral dialog on a clay page reads as a
bug), and the boundary must be re-litigated for every new component. Costs more in judgement
calls than A1 costs in cva edits. Not recommended.

**Decision required: proceed on A1.** Everything below assumes A1; sections B/C/D change
materially under A2.

---

## B. Token / theme foundation

`app/globals.css` (~250 lines) is the highest-blast-radius file in the migration: every component
and both route trees depend on it. It gets rewritten once, early, and verified before any
component is touched.

### B.1 Target shape (confirmed against current shadcn theming docs)

```
@import "tailwindcss";
@import "tw-animate-css";            /* new — makes animate-in/fade-in/zoom-in real */

@custom-variant dark (&:is(.dark *));

@theme inline {                       /* maps bare names -> utilities */
  --color-background: var(--background);
  ... all shadcn semantic tokens ...
  --color-sky-soft: var(--sky-soft);  /* candy palette, same pattern */
  --radius-sm: calc(var(--radius) * 0.6);
  --radius-md: calc(var(--radius) * 0.8);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) * 1.4);
}

:root { --radius: 1.25rem; --background: ...; --sky-soft: ...; --clay-shadow: ...; }
.dark { /* deferred — see B.4 */ }
```

`@theme inline` (not plain `@theme`) is required: `inline` inlines the value into the generated
utility so a `:root` / `.dark` override actually takes effect. Plain `@theme` bakes the value and
dark overrides are ignored.

### B.2 The candy palette must stay Tailwind-visible — non-obvious constraint

Four modules consume candy colours as **string class names**, not as CSS:

| File | Symbol | Emits |
|---|---|---|
| `lib/learn.ts:191` | `phaseTone` | `bg-grape-soft`, `text-grape-ink`, `ring-grape/30` |
| `components/MetricsPanel.tsx:29` | `toneClasses` (7 tones) | `bg-mint-soft`, `text-mint-ink`, ... |
| `components/TaskFlowDiagram.tsx:29` | `toneClasses` (4 tones) | `bg-grape-soft`, `text-grape-ink`, ... |
| `components/ActivityLog.tsx:14` | `eventVariantMap` | 6 of the 8 Badge variant names |

If candy vars stop being theme-utility-generated (e.g. moved to a bare `:root` block only), those
utilities cease to exist. **Nothing fails to compile.** The UI just goes colourless at runtime.
All 25 candy vars (8 hues x base/ink/soft, plus `--indigo-ink`) must therefore be mapped in
`@theme inline`, using the docs' "Adding New Tokens" pattern.

### B.3 hex -> oklch: convert semantics, defer candy

- **Convert** the ~14 shadcn semantic tokens (`--background`, `--foreground`, `--primary`,
  `--card`, `--border`, `--input`, `--ring`, `--muted*`, `--accent*`, `--destructive*`,
  `--popover*`, `--secondary*`) to oklch. Small set, matches docs, and oklch is what makes a
  future dark theme interpolate sanely.
- **Keep** the 25 candy vars as hex for now. They are exact brand values with hand-tuned `-ink`
  contrast (documented >=4.5:1 on white); a 25-value colour-space conversion in the same commit
  as a structural rewrite is avoidable risk, and hex is perfectly legal in `:root`. Convert later
  as an isolated, visually-diffed change if desired.

### B.4 Dark mode: wire the plumbing, defer the design

Add `@custom-variant dark (&:is(.dark *))` and an **empty-but-present** `.dark` block. Do **not**
author a dark palette in this migration.

Consequence to accept explicitly: freshly generated shadcn components ship `dark:` classes (e.g.
`dark:bg-input/30`, `dark:ring-destructive/40`). With the variant declared but no `.dark` class
ever applied to `<html>`, those classes compile and simply never match. That is inert and
harmless — **not** a bug to chase. Designing a dark clay palette (25 candy vars need dark `-soft`
tints, and all three clay shadow recipes are built on white inner highlights that read wrong on
dark surfaces) is a separate design task, out of scope (G).

### B.5 How clay should be expressed

Keep `.clay`, `.clay-sm`, `.clay-inset`, `.clay-btn`, `.clay-chip` as **plain CSS classes** in
`globals.css`. Do not convert to `@utility`.

Rationale: they are composed multi-property recipes with `:hover`/`:active`/`:focus-visible`
states, not single-property utilities; `@utility` buys nothing and loses the state selectors.
Plain classes also pass through `cn()` untouched (`twMerge` has no conflict rules for them, which
is what we want). The cva variant maps in the regenerated primitives then just reference
`clay-btn` / `clay-chip` in their base string — exactly as the current fakes do.

Also carried over verbatim: the 3 shadow recipes, the body radial-gradient background, the
`h1-h4`/`.font-display` Fredoka rule, `clay-pulse`/`.worker-pulse`, `float-soft`/`.float-soft`,
the `.activity-log` scrollbar block, `.toast`, and the `prefers-reduced-motion` block.

### B.6 Tokens to add / reconcile

| Token | Action |
|---|---|
| `--radius-sm/md/lg/xl` | **Add** (derived from `--radius: 1.25rem`) — generated components rely on them |
| `--chart-1..5` | Add placeholders mapped to candy hues; harmless, unblocks future charts |
| `--sidebar-*` | **Skip** — no sidebar in this app, don't carry dead tokens |
| `--destructive-foreground` | **Keep** — our own variants use it; note stock new-york destructive uses `text-white` instead, so don't expect regenerated components to consume it |
| `--secondary` / `--secondary-foreground` | Already present and correctly named; no change |

---

## C. `shadcn add` list

### C.0 First, pin the registry — do this before adding anything

Current shadcn docs expose **three parallel registries** with different peer dependencies:
`components/radix/*` (-> `radix-ui`), `components/base/*` (-> `@base-ui/react`), and
`components/aria/*`. `components.json` here has no `registries` field, so resolution depends on
the installed CLI version.

**Task C0:** add one component (`button`) and inspect the generated import plus the diff to
`package.json`. Whatever it resolves to — Radix or Base UI — **pin that choice for the entire
migration and record it in this file.** Mixing primitive libraries inside `components/ui/` means
two focus-management models, two portal implementations, and two sets of data-attributes to
style. This is a one-command check that prevents a very expensive mistake.

Also expected from the first add: `tw-animate-css` gets installed and `globals.css` may be
auto-patched — which is exactly why **B runs first** (M1), so we control that file, not the CLI.

### C.1 Replacements for the 7 counterfeits — these OVERWRITE

Each regenerates a file that currently exists and currently carries clay styling. `shadcn add`
will overwrite it. **The clay styling in each must be re-applied to the freshly generated
component as cva variants — not left behind.** Diff old against new before deleting anything.

| Add | Overwrites | Clay to re-apply | Unblocks |
|---|---|---|---|
| `button` | `ui/button.tsx` | `clay-btn` base; 5 variants (default/secondary/destructive/outline/ghost); sizes h-10/h-9/h-12/icon; `font-display` | `SiteNav`, `app/learn/*`, `TaskSubmissionPanel` — gains **`asChild`** |
| `card` | `ui/card.tsx` | `clay` base; `p-5 pb-3` header, `p-5 pt-0` content, `font-display` title | 7 panels; gains `CardDescription`/`CardFooter`/`CardAction` |
| `badge` | `ui/badge.tsx` | `clay-chip` base; **8** variants (see 7.3) | `ActivityLog`, `QueuePanel`, `TaskFlowDiagram`, `NodePanel`, `FailedTasksPanel` |
| `input` | `ui/input.tsx` | `clay-inset` + the custom focus-visible inset+ring box-shadow | `TaskSubmissionPanel` (12 fields) |
| `label` | `ui/label.tsx` | `font-display text-xs font-semibold text-foreground/80` | `TaskSubmissionPanel`; gains real Label primitive |
| `dialog` | `ui/dialog.tsx` | `clay` on `DialogContent` | `TaskSubmissionPanel` x2 — **API change, see M3** |
| `progress` | `ui/progress.tsx` | `clay-inset` track + `from-mint to-aqua` gradient indicator | `MetricsPanel` |

### C.2 Genuinely new

| Add | Replaces | File |
|---|---|---|
| `accordion` | 100-line hand-rolled disclosure (`useState<Set<number>>` + manual `aria-expanded`/`aria-controls`) | `components/learn/RevisionQuestions.tsx` |
| `table` | raw `<table>` markup, twice | `FailedTasksPanel.tsx`, `learn/MarkdownContent.tsx` |
| `sonner` | hand-rolled toast (`useState<{type,msg}>` + `setTimeout` + `.toast` class) | `TaskSubmissionPanel.tsx` |
| `tooltip` | 2 of the 4 native `title=` attributes | `NodePanel.tsx` |
| `separator` | `border-t border-border pt-3` ad-hoc dividers | `TaskSubmissionPanel`, `QueuePanel` |
| `skeleton` | `<p>Loading...</p>` | `MetricsPanel`, `QueuePanel` |
| `alert` | `<p className="text-sm text-destructive">{error}</p>` | `MetricsPanel`, `FailedTasksPanel` |
| `form` + `field` | *optional, M3.4* — only if the 12 uncontrolled numeric inputs justify RHF+zod | `TaskSubmissionPanel` |

**Deliberately NOT adding:**

- **`scroll-area`** for `.activity-log` — `ActivityLog` attaches `scrollRef` + `onScroll` directly
  to the scrolling div to drive auto-scroll-to-top. ScrollArea relocates the scrollport into an
  inner viewport, so both the ref and the handler break silently. The existing custom-scrollbar
  CSS already looks right. Not worth the regression (7.5).
- **`accordion` for `InterviewSummary`** — it is a native `<details>`: already accessible,
  zero-JS, and works as an RSC with no `"use client"`. Converting would force it client-side for
  no gain. Leave it.
- **`navigation-menu`** for `SiteNav` — two links do not need a menu primitive with its own
  focus/roving-tabindex model. `Button asChild` + `Link` is correct and simpler. Revisit only if
  a dropdown or mobile `sheet` drawer is actually wanted (G).

---

## D. Ordered tasks

Sequenced so **the app builds and renders at every task boundary.** 24 tasks / 7 milestones.

Verification for every task, minimum: `npx tsc --noEmit` and `npm run build`, **plus a browser
screenshot of the affected surface** (see 7.1 — tsc is blind to the real risk).

### M0 — Branch hygiene (blocking; see F)

| # | Task | Files | Verify |
|---|---|---|---|
| 0.1 | Commit the learn-section work as its own reviewable commit(s) | 11 paths incl. `app/learn/`, `components/learn/`, `SiteNav.tsx`, `content/`, `lib/learn*.ts`, `components.json`, `app/layout.tsx`, `package.json` | `git status` clean |
| 0.2 | Land it to `main`, then cut `feature/shadcn-migration` off updated `main` | — | `git log --oneline -1`; branch confirmed |

### M1 — Token foundation (highest blast radius; nothing else starts until green)

| # | Task | Files | Verify |
|---|---|---|---|
| 1.1 | Add a `typecheck` script (`tsc --noEmit`) — **there is currently no `lint` and no `test` script**, so this is the only automated gate the frontend has | `package.json` | `npm run typecheck` |
| 1.2 | Rewrite `globals.css` to the B.1 target: `@custom-variant dark`, `:root` (semantics in oklch, candy in hex, clay recipes), `@theme inline` (all semantics + **all 25 candy vars** + derived radius), empty `.dark`, all animations/body/scrollbar/reduced-motion carried over | `app/globals.css` | Screenshot `/` and `/learn` — **pixel-identical to pre-change**. Any colour loss = a missing `@theme inline` mapping |
| 1.3 | Install `tw-animate-css` and import it; confirm `animate-in fade-in zoom-in-95` now resolve | `app/globals.css`, `package.json` | The (still-fake) dialog visibly animates for the first time |

> **Gate G1:** `/`, `/learn`, and one chapter page are visually unchanged. Fix any drift here —
> every later task inherits this file.

### M2 — Replace the 7 counterfeits, re-clay each

| # | Task | Files | Verify |
|---|---|---|---|
| 2.1 | **C.0 registry probe** — add `button`, inspect import + `package.json` diff, pin Radix-vs-BaseUI, record the decision in this file | `ui/button.tsx`, `package.json` | Decision written down |
| 2.2 | Re-apply clay to generated `button`: `clay-btn` base, 5 variants, 4 sizes, `font-display`. Watch the `ghost` variant's `!shadow-none` against stock `shadow-xs` (7.4) | `ui/button.tsx` | All buttons on `/` unchanged; hover-lift, active-press, focus ring intact |
| 2.3 | Add `card`, re-apply `clay` + `p-5` rhythm + `font-display` title | `ui/card.tsx` | All 7 panels unchanged |
| 2.4 | Add `badge`, re-apply `clay-chip` + **all 8** variants (7.3) | `ui/badge.tsx` | `ActivityLog` shows 6 distinct colours; `TaskFlowDiagram` 3; `NodePanel` 2 |
| 2.5 | Add `input` + `label`, re-apply `clay-inset` and the custom focus box-shadow | `ui/input.tsx`, `ui/label.tsx` | All 12 fields; focus ring correct; label-to-input association still works |
| 2.6 | Add `progress`, re-apply `clay-inset` track + mint-to-aqua gradient indicator | `ui/progress.tsx` | Success-rate bar identical; now exposes `role="progressbar"` + `aria-valuenow` |
| 2.7 | Add `dialog`, apply `clay` to `DialogContent`. **Do not touch call sites yet** — old and new coexist for one task | `ui/dialog.tsx` | Builds; call-site migration is 3.1 |

> **Gate G2:** `/` is visually identical to G1, and `git diff` shows `@radix-ui/*` (or
> `@base-ui/*`) present in `package.json`.

### M3 — Behavioural migrations in `TaskSubmissionPanel.tsx` (331 lines, riskiest file)

| # | Task | Files | Verify |
|---|---|---|---|
| 3.1 | Migrate both dialogs. **Structural, not a rename:** `onClose` -> `onOpenChange`; `DialogHeader/Title/Description/Footer` must move *inside* a new `DialogContent` (today they are direct children of `Dialog`) | `TaskSubmissionPanel.tsx` | Flush + batch dialogs open/close; **Escape closes**; focus trapped; focus returns to trigger; overlay click closes; `role="dialog"` + `aria-modal` present; title announced |
| 3.2 | Replace hand-rolled toast with `sonner`: delete `toast` state, `showToast`, `setTimeout`; mount `<Toaster />` in `app/layout.tsx`; theme it clay via `toastOptions.classNames` (mint-soft success / coral-soft error) — **not free, sonner ships neutral** | `TaskSubmissionPanel.tsx`, `app/layout.tsx` | All 6 toast call sites fire; success mint, error coral; stacking works |
| 3.3 | Swap the two ad-hoc `border-t` dividers for `Separator` | `TaskSubmissionPanel.tsx`, `QueuePanel.tsx` | Visually identical |
| 3.4 | *Optional* — `form`+`field`+RHF+zod for the 12 numeric inputs, replacing 12 `useState`s and `min`/`max` attrs with schema validation | `TaskSubmissionPanel.tsx` | Invalid input shows inline error; submit still works |

### M4 — New component adoption

| # | Task | Files | Verify |
|---|---|---|---|
| 4.1 | `RevisionQuestions` -> `Accordion type="multiple"`. Deletes `useState<Set<number>>`, `toggle()`, manual `aria-expanded`/`aria-controls`, and the manual chevron rotation. Preserve the numbered `clay-chip` badge and the Show/Hide chip | `components/learn/RevisionQuestions.tsx` | Multiple items open at once; Enter/Space/arrow keys work; screen reader announces expanded state |
| 4.2 | `FailedTasksPanel` raw table -> shadcn `Table`. **Mind the naming trap:** `TableHeader`=`<thead>`, `TableHead`=`<th>`, `TableCell`=`<td>` (7.6) | `FailedTasksPanel.tsx` | 5 columns render; horizontal scroll retained; destructive Badge in Reason column |
| 4.3 | `MarkdownContent` table overrides -> shadcn Table primitives. Higher risk: these are react-markdown render-props receiving only `children`, and shadcn `Table` wraps `<table>` in a `<div>` — verify the resulting DOM is still valid table markup | `components/learn/MarkdownContent.tsx` | A chapter with a GFM table renders correctly and keeps its `clay-inset` frame |
| 4.4 | `NodePanel`: `Tooltip` for the **2 explanatory** `title=` attrs only ("tasks currently leased...", "executor goroutines"). Keep native `title` on the 2 truncated identifiers (`hostname`, `id`) — native is better for truncation reveal. Tooltip triggers must be focusable (7.7) | `NodePanel.tsx` | Tooltip appears on hover **and** on keyboard focus; truncated names still reveal on hover |
| 4.5 | Loading states -> `Skeleton` | `MetricsPanel.tsx`, `QueuePanel.tsx` | Skeleton shows before the first poll resolves |
| 4.6 | Error states -> `Alert variant="destructive"` | `MetricsPanel.tsx`, `FailedTasksPanel.tsx` | Stop the backend: alert renders with `role="alert"` |

### M5 — Nav + learn pages

| # | Task | Files | Verify |
|---|---|---|---|
| 5.1 | `SiteNav`: `<Link className="clay-btn ...">` -> `<Button asChild variant={active?"default":"outline"}>`. Preserve `aria-current="page"` and the active/inactive colour split | `components/SiteNav.tsx` | Active state correct on `/` and `/learn/*`; sticky + backdrop-blur intact |
| 5.2 | `app/learn/page.tsx`: `ChapterCard` -> `Card`; available branch -> `Button asChild` or link-wrapped Card; phase chips and status chips -> `Badge`. Replace `opacity-55` with a proper disabled treatment alongside the existing `aria-disabled` | `app/learn/page.tsx` | Grid unchanged; unavailable cards non-interactive and announced disabled; `phaseTone` colours still correct |
| 5.3 | `app/learn/[slug]/page.tsx` (9 clay usages): `ComingSoonState` -> `Card` + `Button asChild`; `KeyTakeaways` frame stays `clay-inset`; `ChapterNavFooter` prev/next -> `Card`/`Button asChild`; `BackLink` -> `Button variant="ghost" asChild` | `app/learn/[slug]/page.tsx` | Chapter page unchanged; prev/next work; coming-soon state intact |
| 5.4 | *Optional* — `Breadcrumb` for Learn / Phase / Chapter, replacing the single `BackLink` | `app/learn/[slug]/page.tsx` | Breadcrumb navigates correctly |

### M6 — Cleanup + final gate

| # | Task | Files | Verify |
|---|---|---|---|
| 6.1 | Delete now-dead CSS: `.toast` (superseded by sonner). **Audit before deleting** `.clay-btn`/`.clay-chip` — still used directly by non-component markup | `app/globals.css` | `grep` shows zero references before each deletion |
| 6.2 | Reconcile `--destructive-foreground` against stock new-york `text-white`; drop any token with zero references | `app/globals.css` | Build clean |
| 6.3 | Full quality gate + reduced-motion + keyboard-only pass over `/`, `/learn`, one chapter | — | `typecheck` + `build` green; screenshots; `prefers-reduced-motion` honoured; full keyboard traversal |
| 6.4 | Update `PROGRESS.md` session log + `~/.jarvis/projects/task-queue/` state | `PROGRESS.md`, `project.yaml`, `history.md` | Committed |

---

## E. Regression & impact analysis

### 7.1 The primary risk: the type checker is blind to it

`package.json` has **no `lint` and no `test` script** — only `dev`/`build`/`start`. The only
automated signal is `tsc --noEmit` (added in 1.1) and `next build`.

And the most likely breakage is **invisible to both**: `phaseTone`, the two `toneClasses` maps,
and `eventVariantMap` produce Tailwind class names as *strings*. Drop a `@theme inline` mapping or
rename a candy var and each becomes a no-op class. TypeScript is happy, the build succeeds, and
the UI silently loses its colour.

-> **Screenshot verification per milestone is mandatory, not optional.** Gates G1 and G2 exist
specifically to catch this while the diff is still small.

### 7.2 What visually breaks if clay re-styling is missed

| Missed | Symptom |
|---|---|
| `clay` on Card | All 7 panels go flat — no outer drop, no inner highlight. Most visible regression in the app |
| `clay-btn` on Button | Buttons stop lifting on hover / pressing on active; focus ring reverts to stock |
| `clay-chip` on Badge | Every chip flattens; `MetricsPanel` StatCards and `TaskFlowDiagram` stages lose depth |
| `clay-inset` on Input/Progress | Fields and the progress track look raised instead of carved |
| `font-display` | Fredoka lost on buttons/labels/titles -> Nunito everywhere, generic feel |
| A `@theme inline` candy mapping | Colourless chips, **no error anywhere** (7.1) |

### 7.3 Badge: 8 variants, but stock ships 4

Stock Badge (radix registry) has `default | secondary | destructive | outline`. This project uses
**eight**: `default, secondary, destructive, success, warning, info, accent, outline` — and 6 of
them are dispatched dynamically through `ActivityLog`'s `eventVariantMap` (submitted, started,
completed, failed, retrying, dead_lettered, promoted, reclaimed, node_dead, redriven).

Regenerating Badge without re-adding `success`/`warning`/`info`/`accent` means those four variant
strings resolve to nothing, cva falls back to base styling, and the activity log loses its colour
coding — which is that panel's entire pedagogical point. **`ui/badge.tsx` is the single most
important file to diff carefully in M2.**

### 7.4 `!shadow-none` interaction

The current ghost Button and outline Badge use `!shadow-none` to punch through `clay-btn` /
`clay-chip`. Real shadcn components carry their own `shadow-xs`. `twMerge` does have shadow
conflict rules, but `!important`-flag handling has varied across versions — verify the ghost
button and outline badge actually render flat; do not assume.

### 7.5 ScrollArea would break `ActivityLog` (why it is excluded)

`ActivityLog` puts `ref={scrollRef}` and `onScroll={handleScroll}` on the `.activity-log` div and
reads `scrollTop` to decide whether to auto-scroll to top. Radix ScrollArea moves the real
scrollport into an inner viewport node — the ref then points at a non-scrolling wrapper and the
handler never fires. Auto-scroll dies silently. Excluded by design (C.2).

### 7.6 shadcn Table's naming trap

`TableHeader` = `<thead>`, `TableHead` = `<th>`, `TableCell` = `<td>`, `TableRow` = `<tr>`.
Getting `TableHead`/`TableHeader` backwards produces invalid table DOM that React will not
complain loudly about. Affects 4.2 and especially 4.3, where the mapping happens inside
react-markdown's `components` record.

### 7.7 Tooltip accessibility — a real trap, not a formality

The 4 `title=` attributes in `NodePanel` sit on `<span>` and `<code>` elements. Those are not
focusable, so a Radix Tooltip wrapping them is **keyboard-inaccessible** and touch-inaccessible —
strictly worse than the native `title` it replaced. Triggers need `tabIndex={0}` (or a genuinely
focusable element) plus an accessible name. This is why 4.4 converts only the 2 explanatory
tooltips and leaves native `title` on the 2 truncation reveals.

### 7.8 Accessibility wins (the real justification for this migration)

| Today | After |
|---|---|
| Dialog: plain `<div>` — no portal, no focus trap, no Escape, no `role="dialog"`/`aria-modal`, no labelled-by | Real modal semantics, focus trap, focus restore, Escape, scroll lock |
| Progress: plain `<div>` — no `role="progressbar"`, no `aria-valuenow` | Announced progress |
| RevisionQuestions: hand-wired `aria-expanded`/`aria-controls` over a `Set<number>` | Accordion primitive, correct semantics, arrow-key navigation |
| Label: bare `<label>` | Real Label primitive with proper control association |
| Tooltips: native `title` (inconsistent SR support, no touch) | Real tooltip semantics on the explanatory two |
| Toast: a `<div>` appears, no live region | `sonner` announces via a live region |
| Button-as-link: `<Link>` painted to look like a button | `Button asChild` — correct element, correct semantics |

### 7.9 `framer-motion` interaction

`motion` / `AnimatePresence` are used in `NodePanel`, `ActivityLog`, `QueuePanel`, and
`TaskFlowDiagram` — **none inside a Radix/Base UI portal**, so there is no conflict with primitive
mount/unmount. Do **not** add `AnimatePresence` exit animations inside `DialogContent`: the
primitive controls unmount and will race it. Dialog animation should come from `tw-animate-css`
(`data-[state=open]:animate-in`), which task 1.3 finally makes functional.

### 7.10 Behaviour changes a user will notice

- Dialogs now trap focus and close on Escape (an improvement, but different).
- Toasts move to sonner's stacking/positioning; the fixed bottom-right `.toast` recipe goes away.
- Revision questions become a real accordion — arrow keys now navigate between items.
- Nav links become button-styled anchors via `asChild` (semantics unchanged, focus ring may differ).
- Truncated node names keep native `title`; the two explanatory hints become hover/focus tooltips.

---

## F. Branch hygiene — resolve before task 1.1

**Current state:** on `feature/learn-section` with substantial **uncommitted** work:

```
 M frontend/app/layout.tsx
 M frontend/package.json
 M frontend/package-lock.json
?? frontend/app/learn/          ?? frontend/components/learn/
?? frontend/components.json     ?? frontend/components/SiteNav.tsx
?? frontend/content/            ?? frontend/lib/learn.ts
?? frontend/lib/learnProgress.ts
```

That is an entire unreviewed feature — 6 new components, 2 new routes, 2 new lib modules, and
content — sitting in the working tree.

**Recommendation:** commit and land the learn section first (M0), then run this migration on
`feature/shadcn-migration` cut from updated `main`.

**Risk if ignored:** this migration touches ~20 files including `globals.css` and all 7 files in
`components/ui/`. Overlaying a wide mechanical refactor on unreviewed feature work means:

1. The eventual diff is unreviewable — nobody can separate a learn-section bug from a migration bug.
2. `shadcn add` **overwrites** files. Overwriting an uncommitted `ui/*.tsx` is unrecoverable —
   there is no committed version to `git diff` the clay styling out of. The M2 instruction
   "diff old against new" becomes impossible.
3. No rollback: if the `globals.css` rewrite goes wrong, `git checkout` also discards the learn work.
4. `components.json` and `package.json` are both untracked/modified *and* about to be modified by
   the CLI — guaranteed confusion about which change came from where.

Point 2 is the hard blocker. **Do not run `shadcn add` while `components/ui/` is uncommitted.**

---

## G. Not in scope

- **No backend changes.** Go services, Redis, Docker untouched. `lib/api.ts` and `lib/types.ts` unchanged.
- **No new features.** Zero user-facing capability added; this is a component-layer and theme-layer refactor.
- **No redesign of the visual language.** Claymorphism, the candy palette, Fredoka/Nunito, and the
  radial-gradient background are preserved as-is. Not a visual refresh.
- **No dark-mode design.** Plumbing only (`@custom-variant` + empty `.dark`). Authoring a dark clay
  palette — 25 candy vars need dark `-soft` tints, and all 3 clay shadow recipes depend on white
  inner highlights that read wrong on dark surfaces — is its own design task.
- **No hex-to-oklch conversion of the candy palette** (semantics only; B.3).
- **No `scroll-area`** (7.5), **no `navigation-menu`**, **no mobile `sheet` drawer** — `SiteNav`
  currently has no mobile treatment at all; adding one is a feature, not a migration.
- **No `InterviewSummary` conversion** — native `<details>` is already correct.
- **No PROGRESS.md phase work.** Phase 3 (slog / Prometheus / Grafana / BZPOPMIN) is untouched and
  still next after this.
- **No test suite.** There is no test infrastructure; adding one is a separate decision. Task 1.1
  adds only a `typecheck` script.

---

## Open questions for the ★ PLAN gate

1. **Section A — confirm A1** (shadcn behaviour + clay theme). A2/A3 change everything downstream.
2. **Section F — land the learn section first?** Strongly recommended; task 2.1 must not run otherwise.
3. **C.0 — Radix or Base UI?** Determined by probe, but confirm that whichever the CLI picks is
   acceptable to standardise on.
4. **Optional tasks 3.4 and 5.4** — include `form`+`field` (RHF+zod) and `Breadcrumb`, or defer?
5. **B.4 — dark mode:** plumbing-only now (recommended), or is a real dark theme wanted in this pass?
