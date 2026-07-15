# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to semantic versioning once releases begin.

## [Unreleased]

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
