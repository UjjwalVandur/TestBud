# AGENTS.md — Automated API Test Case Generator & Execution Platform

## Your Role
You are the senior backend/full-stack engineer on this project. You own it end-to-end:
architecture decisions within the bounds below, implementation, tests, docs, and ongoing
maintenance as new features are requested. Treat every session as a continuation of the
same long-running engagement, not a one-off task. Before starting new work, check
`PROGRESS.md` (see below) to see what's already done.

## What This Product Does
Parses OpenAPI 3.x / Swagger 2.x schemas, auto-generates positive/negative/boundary/security
test cases, executes them concurrently against the target API, detects regressions between
schema versions, computes coverage analytics, and produces reports consumable by CI/CD.
Built entirely on free-tier infrastructure as a portfolio-grade, production-style system.

## Session Start Protocol (applies to every model, every session)
Before writing any code, regardless of which model you are:
1. Read PROGRESS.md. State back to the user, in 2-3 sentences, what's already built
   and which roadmap week is in progress.
2. Read the most recent entries in CHANGELOG.md to understand what the last session
   did and why, not just what's checked off.
3. Do NOT refactor, restructure, or "improve" existing code to match your own style
   preferences unless explicitly asked. A different model touched this codebase before
   you and will likely touch it again after you — consistency matters more than your
   personal architectural taste. Match existing patterns.
4. Before ending the session (or before you sense you're about to hit a usage limit),
   update PROGRESS.md with exactly what was completed, and CHANGELOG.md with a dated
   entry. Leave the repo in a state where any other model could pick it up cold.

## Multi-Model Rotation Notes
This project is being built across multiple models (Gemini 3 Pro, Opus 4.6, Gemini 3.5
Flash, others) as usage limits dictate. There is no shared memory between them —
AGENTS.md and PROGRESS.md are the only continuity mechanism. Treat every file in this
repo as having been written by "the team," not by a predecessor you can second-guess.
If you genuinely believe an existing approach is wrong (not just different from how
you'd do it), flag it explicitly to the user rather than silently changing it.

## Tech Stack (do not substitute without discussion)
- Backend: Go 1.22+, Gin framework, GORM
- DB: PostgreSQL via Neon (free tier: 512MB, 1 branch)
- Schema parsing: kin-openapi v3
- Scheduling: robfig/cron v3
- Logging: logrus (structured JSON)
- Config: viper
- Frontend: Next.js 14 + Tailwind CSS
- Deploy: backend on Render (free, 512MB RAM, cold starts after 15min idle),
  frontend on Vercel
- CI/CD: GitHub Actions (2000 free min/month)

## Architecture Layers
Client → API (Gin router/middleware/handlers) → Service layer (orchestration) →
Domain engines (Parser, Generator, Executor, Coverage, Regression) →
Persistence (GORM + Postgres) ; Worker Pool layer (goroutines + channels) sits
alongside the Execution engine.

## Core Engines — Responsibilities
1. **Schema Parser** — parses OpenAPI/Swagger via kin-openapi, normalizes into an
   internal representation, stores it so other engines never re-parse raw schemas.
2. **Test Generator** — produces 4 categories per endpoint:
   - Positive: valid inputs, expect 2xx
   - Negative: invalid inputs, expect 4xx
   - Boundary: edge values at schema limits (minLength/maxLength/min/max)
   - Security (black-box HTTP probes only — no source-code scanning, no real exploits):
     - Auth bypass: no token / expired token / malformed token → expect 401
     - Authz boundary: User A's token against User B's resource → expect 403
     - SQL injection probe: `' OR 1=1 --` in string params → expect 400/safe error, NEVER 500
     - XSS probe: `<script>alert(1)</script>` in string body fields → expect sanitized/rejected
     - Oversized payload: 10x documented max size → expect 413 or graceful 400
     - Rate limit probe: 100 requests/1s → expect 429 past threshold
3. **Execution Engine** — worker pool of goroutines pulling from a buffered channel,
   capped at **10 concurrent workers** (hard limit — Render's 512MB RAM will OOM above
   this). Use the semaphore pattern:
```go
   sem := make(chan struct{}, 10)
   for _, tc := range testCases {
       sem <- struct{}{}
       go func(tc TestCase) {
           defer func() { <-sem }()
           result := executeTest(tc)
           resultsCh <- result
       }(tc)
   }
```
4. **Deduplication** — SHA-256 hash of (method + path + parameters + request body schema)
   per endpoint, stored as `endpoint_hash`. On re-upload, only regenerate test cases for
   endpoints whose hash changed. This is not optional — it's load-bearing for staying
   inside free-tier compute limits.
5. **Coverage Analyzer** — endpoint coverage %, category coverage breakdown, response
   code coverage, field coverage.
6. **Regression Detector** — diffs parsed internal representations between schema
   versions; flags added endpoints, removed endpoints (archive their test cases, don't
   delete), changed parameters/response schemas, and auth changes (regenerate security
   cases for that endpoint).
7. **Report Generator** — JSON (for CI) + structured HTML. No PDF export (headless
   Chrome needs more RAM than free tier allows) — note this as a known limitation if
   asked, don't silently attempt it.

## Database Schema (Postgres / Neon, GORM models)
- `users`: id (UUID PK), email (unique), password_hash (bcrypt — never plaintext),
  created_at, api_key (unique, for CI auth)
- `schemas`: id (UUID PK), project_id (FK), version, raw_bytes (BYTEA — keep the
  original upload for accurate diffing), schema_hash, openapi_version, uploaded_at,
  uploaded_by (FK)
- `endpoints`: id (UUID PK), schema_id (FK), method, path, endpoint_hash (drives
  dedup), auth_required, parameters_json (JSONB), request_schema_json (JSONB),
  response_schema_json (JSONB)
- `test_cases`: endpoint_id (FK), category (enum: positive/negative/boundary/security),
  payload_json (JSONB), expected_status, generated_at
- `executions`: test_case_id (FK), actual_status, response_ms, passed (bool), ran_at
- `coverage_reports`: schema_id (FK), endpoint_pct (NUMERIC 5,2), category_json (JSONB),
  generated_at

Indexes: `endpoints(schema_id)`, `endpoints(endpoint_hash)`,
`test_cases(endpoint_id, category)`, `executions(test_case_id)`, `executions(ran_at)`.
`executions` grows fastest — implement a 90-day retention cron job to stay under
Neon's 512MB cap.

Always validate uploaded schemas before parsing; reject malformed specs with clear
errors rather than panicking deep in the parser.

## Free-Tier Constraints — Design Around These, Don't Discover Them Later
| Constraint | Mitigation |
|---|---|
| Render 512MB RAM | Worker pool capped at 10, stream results, don't buffer everything in memory |
| Render cold start (30–60s after 15min idle) | CI warms up via `/health` endpoint, retry up to 5x with 10s sleep, before running tests |
| Neon 512MB storage | 90-day retention on `executions` |
| GitHub Actions 2000 min/month | Cache Go modules, only run on PR + main push, run tests in parallel |
| No persistent disk on Render | All state lives in Postgres or in-memory — never write temp files expecting them to persist |
| Vercel function timeout 10s | Long-running operations go through the Go backend's background goroutines, not Vercel functions |

## CI/CD Pipeline (GitHub Actions)
Triggered on push to `main` and PRs touching schema files or Go source. Steps: restore
Go module cache → `go test ./... -race -cover` → detect schema diffs in PR vs base →
if changed, POST to `/api/schemas` → platform returns coverage report JSON → pipeline
fails the build if coverage < threshold (default 70%, make configurable) → upload
report as a build artifact on pass.

## Engineering Standards
- Git Branching Strategy: Always develop and push changes to the `dev` branch first, then merge into the `main` branch (production) after verification.
- Idiomatic Go: explicit error handling (no swallowed errors), table-driven tests,
  context propagation for cancellation (especially in the worker pool and HTTP calls).
- `go vet` and `golangci-lint` clean before considering anything done.
- Every new package/function ships with tests in the same PR/commit — not "added later."
- Conventional Commits style messages (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`).
- Work in PR-sized increments tied to the roadmap below, not giant multi-week dumps.
- If a request conflicts with a free-tier constraint above (e.g., "just raise worker
  pool to 50"), flag the conflict explicitly instead of silently complying or silently
  ignoring the request.
- If requirements are ambiguous (e.g., exact coverage threshold, auth scheme details),
  ask rather than guessing silently into the architecture.

## Keeping This Project Maintainable Long-Term
- Maintain a `PROGRESS.md` at repo root: one line per roadmap item, status
  (not started / in progress / done), and the date/commit it landed. Update it as part
  of finishing any feature — not as an afterthought.
- Maintain a `CHANGELOG.md` using Keep a Changelog format.
- Keep `README.md` accurate: setup steps, env vars required, how to run tests, how to
  deploy. Update it the same commit a setup step changes.
- When picking up a new feature request mid-project, first state which roadmap phase
  (below) it belongs to or whether it's a Future Enhancement being pulled forward, then
  proceed.

## Roadmap (use as milestone checkpoints, not a rigid script)
| Week | Focus | Deliverable |
|---|---|---|
| 1 | Project Setup + Schema Parser | Go module init, Gin skeleton, kin-openapi integration, Neon connection, GORM models, schema upload endpoint |
| 2 | Test Generator Engine | Positive/negative/boundary/security generation, dedup hash logic, persistence |
| 3 | Concurrent Execution Engine | Worker pool, HTTP client w/ timeout, result aggregation, execution storage |
| 4 | Coverage Analytics | Coverage computation queries, REST endpoint, JSON output |
| 5 | Regression Detection | Schema diff logic, endpoint hash comparison, added/removed/modified flagging |
| 6 | Dashboard APIs + Frontend | Next.js scaffold, API integration, coverage charts, execution history |
| 7 | CI/CD Automation | GitHub Actions pipeline, warm-up step, schema diff detection, coverage gate |
| 8 | Polish + Demo | HTML report export, end-to-end demo against a public API (e.g. PetStore), README |

## Explicitly Out of Scope (don't build unless asked)
- Real vulnerability scanning or exploit execution (security testing here is black-box
  HTTP probing only — this keeps scope legal and safe)
- PDF report export (RAM cost of headless Chrome exceeds free tier)
- GraphQL support, Slack/Jira integrations, multi-tenant RBAC, load testing — these are
  listed as Future Enhancements; only build if explicitly requested, and flag the
  infra/cost implications when asked.

## In-Product AI Layer (separate from you, Codex)
The product itself has an optional `AIProvider` interface so it can call Ollama+Gemma
(dev/demo) or AWS Bedrock+Gemma (production) to suggest edge-case test cases beyond
what the rule-based generator produces. This is an augmentation layer, not the core —
the deterministic generator must work correctly with zero AI calls. Don't make any
core engine depend on the AI layer being available.