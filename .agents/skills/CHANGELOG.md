# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to semantic versioning once releases begin.

## [Unreleased]

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
