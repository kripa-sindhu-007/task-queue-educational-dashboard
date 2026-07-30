# CLAUDE.md

## Project

Educational distributed task queue (Go + Redis + Next.js) being evolved over ~1 month
into a **distributed task processing platform**: standalone worker nodes, at-least-once
delivery, heartbeats, leader election, Prometheus observability.

## ⚠️ Start every session here

**Read `PROGRESS.md` first.** It is the single source of truth for:

- the roadmap (Phases 0–4 with stable task IDs like `P1.3`)
- current status (Status Snapshot table at the top)
- what happened in previous sessions (Session Log)
- decisions already made (Decision Log) — do not re-litigate them

Follow the update rules in its "How to Use This File" section: update task statuses as
you work, append a Session Log entry and refresh the Status Snapshot before ending a
session. Reference task IDs in commit messages, e.g. `feat(broker): lease dequeue [P1.2]`.

## Layout

- `backend/` — Go 1.22, stdlib + go-redis only. Entry: `cmd/server/main.go`.
  Packages: `internal/{api,config,model,queue,store,worker}` (more added per PROGRESS.md phases).
- `frontend/` — Next.js 15 dashboard, polls the API (`frontend/lib/api.ts`).
- `docker-compose.yml` — redis + backend + frontend. Run: `docker compose up --build`.

## Conventions

- Keep the stdlib-first philosophy; justify any new dependency in PROGRESS.md's Decision Log.
- Multi-step Redis mutations must be atomic (Lua via `//go:embed`).
- New backend code needs tests (miniredis for units; testcontainers for integration).
- The dashboard is the project's differentiator — new backend behavior (events, states)
  should surface there when reasonable.
