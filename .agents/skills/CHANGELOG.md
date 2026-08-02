# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to semantic versioning once releases begin.

## [Unreleased]

### Added
- **Authentication**: Integrated Clerk for authentication in both the frontend and backend.
  - Replaced APIKeyAuth middleware with a dual-auth middleware supporting Clerk `Bearer` tokens and CLI `X-API-Key` authentication.
  - Implemented lazy user provisioning in the backend, automatically linking users by their `ClerkID`.
  - Upgraded Next.js frontend with `@clerk/nextjs`, added protected routes via `middleware.ts`, and updated the `Header` with login/profile buttons.
  - Added a new `/settings` dashboard page for users to view and copy their API tokens.
- **Bruno (.bru) Support**: Added a new parser that can process Bruno API client files. 
  - Supports uploading `.zip` archives containing multiple `.bru` files as well as individual `.bru` files.
  - Features an intelligent schema inferencer that synthesizes OpenAPI schemas from concrete Bruno request bodies and parameters, enabling seamless test case generation.

### Fixed
- **Docker & Environment Variables**: Added missing Clerk environment variables (`CLERK_SECRET_KEY` and `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`) to `docker-compose.yml` and `web/Dockerfile` so Clerk initializes correctly in the containerized deployment.
- **Frontend Build Stability**: Fixed multiple ESLint and TypeScript build issues (e.g., `react-hooks/exhaustive-deps`, removing unsafe `any` types) ensuring `npm run build` passes successfully.
- Fixed Next.js cascading render warnings (eslint `react-hooks/set-state-in-effect`) in the `/schemas` and `/schemas/[id]/executions` pages.
- Fixed unused variable and `@ts-ignore` linting errors in the `web` folder.

## [1.0.0] - 2026-07-31

### Added
- Docker containerization with `docker-compose.yml` for 1-command local setup:
  - PostgreSQL 16, Go backend, Next.js frontend, and mock target API.
  - Healthcheck-based startup ordering (db → backend → frontend).
- Database seed script (`cmd/seed/main.go`) that creates a demo user with API key `testbud-demo-key-2026` on startup (idempotent).
- Mock Petstore API server (`examples/mock-server/main.go`) using Go stdlib:
  - `GET /pets`, `POST /pets`, `GET /pets/{id}`, `DELETE /pets/{id}` (auth required), `PUT /pets/{id}`.
  - Produces mixed pass/fail results for realistic test execution demos.
- Sample OpenAPI specs for demo:
  - `examples/petstore-v1.yaml` — 4 endpoints with query, path, body params and bearer auth.
  - `examples/petstore-v2.yaml` — modified spec with added PUT, removed DELETE, and changed POST schema for regression demo.
- `DEMO.md` — step-by-step 5-minute walkthrough guide for all features.
- Multi-stage Dockerfiles for backend (`Dockerfile`), frontend (`web/Dockerfile`), and mock server (`examples/mock-server/Dockerfile`).
- Next.js standalone output mode for Docker deployment.
- Optional AI-powered test generation module (`internal/aigenerator`) using Gemma 4 via AWS Bedrock:
  - `CompositeGenerator` pattern merging rule-based and LLM-generated edge cases under `ai_enhanced` category.
  - Per-endpoint rate limiting to prevent Bedrock throttling.
  - Graceful fallback to rule-based cases if Bedrock API is unconfigured or returns an error.

### Changed
- README.md updated to v1.0.0 with Docker Quickstart section and Demo reference.

## [0.7.0] - 2026-07-30

### Added
- CI/CD CLI binary (`cmd/cli/main.go`) for pipeline integration:
  - 3-step workflow: upload schema → execute tests → fetch detailed results.
  - Configurable pass rate threshold via `--fail-threshold` (default 100%).
  - Structured exit codes: 0 (pass), 1 (test failure), 2 (infrastructure error).
  - Formatted terminal output with summary table and failed tests breakdown.
  - Flag parsing for all required and optional parameters (api-url, api-key, project-id, version, schema-file, target-url, auth-headers, alt-auth-headers).
- GitHub Actions workflow template (`.github/workflows/testbud.yml`) demonstrating CI integration.
- 7 unit tests for CLI: flag validation (required/missing/invalid threshold/custom threshold), output formatting (all pass/some fail/custom threshold pass), and missing flags exit code.
- CI/CD Integration section in README.md with full usage docs, flag reference, and exit code table.

## [0.6.0] - 2026-07-23

### Added
- Dashboard REST APIs for schema listing and details:
  - `GET /api/schemas` — list schemas by authenticated user, optional `?project_id=` filter.
  - `GET /api/schemas/:id` — full schema detail with endpoint list and per-endpoint test case counts by category (positive/negative/boundary/security).
  - `GET /api/schemas/:id/executions` — execution history with summary stats (total/passed/failed/avg response time) and detailed log.
- `FindByUploadedBy` and `FindByIDWithDetails` repository queries for the dashboard.
- `GetExecutionsBySchemaID` repository query joining through endpoints → test_cases → executions.
- `SchemaListItem`, `SchemaDetailResult`, `EndpointDetail`, `ExecutionListResult`, `ExecutionItem`, `ExecutionSummary` DTOs.
- CORS middleware (`internal/api/middleware/cors.go`) with configurable origins via `CORS_ORIGINS` env variable.
- Handler interfaces `SchemaLister` and `ExecutionLister` for testability (matching existing handler interface pattern).
- 10 new handler unit tests: 4 for schema List, 3 for schema GetByID, 3 for execution List.
- Next.js 14 frontend (`web/`) with App Router, TypeScript, Tailwind CSS:
  - Dashboard overview page with KPI cards, quick actions, and recent schemas table.
  - Schemas list page with card grid, upload modal with drag-and-drop dropzone.
  - Schema detail page with HTTP method badges, auth indicators, and test case category breakdowns.
  - Execution history page with trigger modal, pass rate progress bar, and detailed execution log.
  - Coverage analytics page with endpoint/category/response code/field coverage visualizations.
  - Regression diff page with added (green), removed (red), and modified (amber) endpoint views.
  - API key settings modal with localStorage persistence.
  - Dark theme with glassmorphism design system, micro-animations, and custom scrollbar.

### Changed
- `RouterDependencies` fields renamed for clarity: `SchemaService` → `SchemaUploader`/`SchemaLister`, `ExecutionService` → `ExecutionExecutor`/`ExecutionLister`.
- `NewSchemaHandler` and `NewExecutionHandler` constructors now accept separate upload/list interfaces.
- Added `CORS_ORIGINS` (comma-separated) to config with default `http://localhost:3000`.

## [0.5.0] - 2026-07-18

### Added
- Regression Detector domain engine (`internal/regression`) that diffs two schema versions' endpoint sets, identifying added, removed, and modified endpoints. Uses canonical JSON normalization to prevent false positives from key ordering or whitespace differences.
- `RegressionService` (`internal/service/regression.go`) coordinating target schema lookup, predecessor discovery via `FindPredecessorSchema`, and diff computation.
- `GET /api/schemas/:id/regression` REST endpoint returning the full regression report with `base_schema_id`, `target_schema_id`, `added`, `removed`, and `modified` arrays.
- `FindByID` and `FindPredecessorSchema` methods on `SchemaRepository` for loading schema by UUID and finding the immediate predecessor by project and upload timestamp.
- Auth-change-aware dedup in schema upload: when an endpoint's `AuthRequired` flag changes but parameters/request/response schemas remain identical, positive/negative/boundary test cases are preserved and only security test cases are regenerated.
- Comprehensive tests: 10 detector unit tests + 6 canonical JSON tests, 4 service tests, 4 handler tests, 1 auth-change dedup test.

## [0.4.0] - 2026-07-16

### Added
- Coverage Analyzer domain engine (`internal/coverage`) computing endpoint coverage %, category coverage breakdown (positive/negative/boundary/security), response code distribution, and field coverage from schema-defined parameters and request body properties.
- `CoverageRepository` (`internal/repository/coverage.go`) with upsert semantics (one row per schema, updated in-place) and `FindBySchemaID` query.
- `GetExecutionsByTestCaseIDs` method on `ExecutionRepository` for efficient bulk execution lookup by test case IDs (uses existing `test_case_id` index).
- `CoverageService` (`internal/service/coverage.go`) orchestrating lazy coverage computation: loads endpoints → test cases → executions, delegates to the analyzer, upserts the report, and returns a JSON-friendly DTO.
- `GET /api/schemas/:id/coverage` REST endpoint returning the full coverage report with `endpoint_pct`, `categories`, `response_codes`, `fields`, and `generated_at`.
- Comprehensive tests: 7 analyzer unit tests, 5 service tests with in-package fakes, 3 handler tests.

## [0.3.0] - 2026-07-13

### Added
- Concurrent Execution Engine (`internal/executor`) that runs generated test cases against a live target API via HTTP, with 10s client timeout and full `context.Context` propagation for cancellation.
- Semaphore-pattern worker pool (`internal/service/executions.go`) capped at 10 concurrent goroutines (Render 512MB RAM hard limit). Results stream to DB as they arrive — no full-buffer.
- Support for all security probe execution modes: auth bypass (`OmitAuth`), authz boundary (`UseOtherUserAuth` with alt credentials), oversized payload generation, and rate limit probing (100 sequential requests).
- `POST /api/schemas/:id/executions` endpoint accepting `target_url`, `auth_headers`, and `alt_auth_headers` in JSON body.
- `ExecutionRepository` with `CreateExecution` (single-insert streaming) and `DeleteOldExecutions` (90-day retention).
- `GetEndpointsWithTestCases` method on `SchemaRepository` to load endpoints with preloaded test cases for execution dispatch.
- Daily 90-day execution retention cron job via `robfig/cron/v3` (schedule: `0 2 * * *`), with graceful shutdown integration.
- Comprehensive executor unit tests (7 tests): path/query/header construction, auth omit, alt auth, oversized probe, rate limit probe, context cancellation, POST with body.
- Execution service tests (5 tests): full run, concurrency cap verification, empty schema, context cancellation, mixed pass/fail aggregation.

### Fixed
- Worker pool deadlock: moved dispatch loop into a separate goroutine so the collector can drain `resultsCh` concurrently, preventing deadlock when the buffered channel fills.

## [0.2.1] - 2026-06-28

### Added
- Granular negative test case generation (DEV-16) to generate separate negative test cases by omitting one required parameter or field at a time while keeping others valid.
- Unit tests verifying edge-cases (DEV-17): `TestGenerator_EmptyEndpoint`, `TestGenerator_MalformedParametersJSON`, `TestGenerator_MalformedRequestSchemaJSON`, `TestGenerator_GranularNegative`, `TestGenerator_ComplexSchema` (enums/arrays/nested objects), and `TestGenerator_ExclusiveBounds`.

### Changed
- Refactored oversized-probe payload generation (DEV-10): replaced 5 MB in-memory string with metadata flags (`IsOversizedProbe` and `OversizedBytes`) to avoid blowing Neon's 512 MB storage limit.
- Handled error paths in security generator `json.Marshal` calls instead of discarding errors (DEV-14).
- Preserved `GeneratedAt` timestamp when copying test cases during endpoint deduplication (DEV-11).

## [0.2.0] - 2026-06-28

### Added
- Rule-based deterministic Test Case Generator Engine (`internal/generator`) producing Positive, Negative, Boundary, and Security test cases.
- Support for type-specific value generators (string, integer, number, boolean, object, array).
- Boundary constraint generation testing limits and edge-values (inclusive and exclusive bounds).
- Security probes for Auth Bypass, Authz Boundary, SQL Injection, XSS, Oversized Payload, and Rate Limit check.
- Endpoint-level deduplication: copies existing test cases from the previous schema's matching endpoint if their `endpoint_hash` is identical.
- GORM recursive association saves: added `TestCases` array relation to `Endpoint` model so schemas, endpoints, and all nested test cases are persisted recursively in a single transaction.
- Comprehensive generator unit tests (verifying positive, negative, boundary, and security outputs).
- Service integration tests verifying copy-on-duplicate endpoint hash and test case persistence.

## [0.1.1] - 2026-06-28

### Added
- API key auth middleware (`X-API-Key` header + `Authorization: Bearer` fallback).
- User repository (`FindUserIDByAPIKey`) for auth middleware lookups.
- GORM models for `test_cases`, `executions`, `coverage_reports` with proper indexes.
- Schema-level dedup: skip re-upload if `schema_hash` + `project_id` already exists.
- Tests for `internal/config` (5 tests), router wiring (3 tests), auth middleware (5 tests).
- Swagger parser test now verifies Method, Path, AuthRequired, ResponseSchemaJSON fields.
- Service tests for `missing uploaded_by` and `missing version` validation branches.
- Dedup service test verifying short-circuit on identical schema hash.

### Changed
- `RouterDependencies.SchemaService` is now typed as interface (`SchemaUploader`), not concrete.
- Schema upload handler derives `uploaded_by` from auth context, not form input (security fix).

### Fixed
- Swallowed error in `endpointHash` (`json.Marshal` result was discarded).
## [0.1.0] - 2026-06-26

### Added
- Initialized Week 1 backend scaffold.
- Added Gin API server with `/health`.
- Added Viper configuration and GORM Postgres connection.
- Added Week 1 GORM models for users, schemas, and endpoints.
- Added OpenAPI schema upload flow with parse-before-persist validation.
- Added parser and upload service tests.
- Added Git Branching Strategy to  Engineering Standards and set up the `dev` branch.
