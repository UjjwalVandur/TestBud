---
name: code-review
description: Full senior Go code review against AGENTS.md spec
---

Read AGENTS.md and PROGRESS.md first. Then do a full code review of the current
codebase as a senior Go engineer. Structure your review in this exact order:

**1. Architecture Conformance**
Check every completed module against the spec in AGENTS.md. Call out any deviation
from the defined layer boundaries (API → Service → Domain Engine → Persistence).
If a handler is doing what a service should do, or a service is doing what an engine
should do — flag it.

**2. Free-Tier Constraint Violations**
Actively look for:
- Worker pool exceeding 10 concurrent goroutines
- Anything writing to disk that won't survive a Render restart
- DB queries that could grow unbounded and blow Neon's 512MB cap
- Missing or incorrect 90-day retention logic on the executions table
Flag each as HIGH (will break in prod) or MEDIUM (will bite later).

**3. Go Correctness**
- Swallowed errors (err assigned but not checked or returned)
- Goroutine leaks (goroutines started with no cancellation path)
- Context not threaded through HTTP calls and worker pool
- Race conditions (flag anything you'd expect go test -race to catch)
- GORM pitfalls: missing transactions where needed, N+1 queries

**4. Security**
- Passwords stored without bcrypt
- API keys committed or hardcoded
- SQL injection surface in any raw query
- Missing input validation on the schema upload endpoint before parsing

**5. Test Coverage**
List every package/file with zero or insufficient tests against what AGENTS.md
requires. Don't praise existing tests — only flag gaps.

**6. Prioritised Fix List**
Finish with a ranked list: P1 (fix before moving to next roadmap week), P2 (fix
this week), P3 (fix before demo). No more than 10 items total.

Do not suggest refactors for style preference. Only flag things that are wrong,
risky, or directly violate the AGENTS.md spec.