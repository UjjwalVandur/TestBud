# TestBud

Automated API Test Case Generator & Execution Platform.

TestBud is a portfolio-grade, production-style platform built entirely on free-tier infrastructure. It parses OpenAPI 3.x / Swagger 2.x schemas, automatically generates positive, negative, boundary, and security test cases, and executes them concurrently against a target live API.

The repository is currently at **Week 5** of the roadmap: **Regression Detection**.

---

## Architecture & Core Modules

1. **Schema Parser**: Uses `kin-openapi` to validate and normalize OpenAPI/Swagger schemas.
2. **Test Generator**: A rule-based, deterministic engine generating positive, negative, boundary, and security test cases per endpoint.
3. **Concurrent Execution Engine**: A worker pool capped at **10 concurrent workers** (Render RAM memory safety ceiling) that pulls test cases and runs them via HTTP client (10s timeout) with full `context.Context` cancellation.
4. **Deduplication Engine**: Endpoint-level hash matching (`endpoint_hash`). Test cases are regenerated only for new or modified endpoints, minimizing free-tier compute usage.
5. **Execution Retention Policy**: Daily cron job running at 2:00 AM using `robfig/cron/v3` to clean up execution logs older than 90 days, ensuring Neon's 512MB storage cap is never exceeded.
6. **Coverage Analyzer**: Computes endpoint coverage %, category coverage breakdown, response code distribution, and field coverage for each schema version.
7. **Regression Detector**: Diffs parsed internal representations between schema versions; flags added, removed, and modified endpoints (parameters, request/response schemas, auth changes). When only `AuthRequired` changes, non-security test cases are preserved and only security cases are regenerated.

---

## Requirements

- Go 1.22 or newer
- PostgreSQL database (e.g. Neon free-tier, 1 branch)

---

## Configuration

Copy `.env.example` to `.env` and configure your credentials:

```env
APP_ENV=development
PORT=8080
DATABASE_URL=postgres://USER:PASSWORD@HOST.neon.tech/DBNAME?sslmode=require
AUTO_MIGRATE=true
```

> [!IMPORTANT]
> Always use Neon's pooled Postgres URL with `sslmode=require`. On startup, GORM auto-migrates all tables: `users`, `schemas`, `endpoints`, `test_cases`, `executions`, and `coverage_reports`.

---

## Running the Platform

### Running Tests

Run all unit and integration tests across the codebase:

```bash
go test ./... -count=1
```

### Starting the Server

```bash
go run ./cmd/api
```

---

## API Endpoints

All `/api/*` endpoints require authentication. Provide your API Key in either:
- `X-API-Key: <your-key>` header
- `Authorization: Bearer <your-key>` header

### 1. Health Check
Checks server viability.

```bash
curl http://localhost:8080/health
```

### 2. Upload OpenAPI Schema
Parses, deduplicates, and generates test cases for an OpenAPI schema.

```bash
curl -X POST http://localhost:8080/api/schemas \
  -H "X-API-Key: your_user_api_key" \
  -F "project_id=00000000-0000-0000-0000-000000000001" \
  -F "version=1.0.0" \
  -F "file=@openapi.json"
```

### 3. Trigger Test Case Executions
Concurrently runs all generated test cases for the specified schema against a live target base URL.

```bash
curl -X POST http://localhost:8080/api/schemas/00000000-0000-0000-0000-000000000001/executions \
  -H "X-API-Key: your_user_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "target_url": "http://localhost:8081",
    "auth_headers": {
      "Authorization": "Bearer valid_target_api_jwt"
    },
    "alt_auth_headers": {
      "Authorization": "Bearer alt_user_target_api_jwt"
    }
  }'
```

#### JSON Payload Attributes:
- `target_url` (string, required): The root URL of the target API instance being tested.
- `auth_headers` (object, optional): Default/valid headers supplied to standard authenticated routes.
- `alt_auth_headers` (object, optional): Alternative headers supplied to check cross-user authorization boundaries (e.g. User B's token sent to check User A's endpoints to expect `403 Forbidden`).

### 4. Get Coverage Report
Computes and returns the coverage analytics for the specified schema based on its latest execution results.

```bash
curl http://localhost:8080/api/schemas/00000000-0000-0000-0000-000000000001/coverage \
  -H "X-API-Key: your_user_api_key"
```

#### Response JSON:
- `schema_id` (string): The schema UUID.
- `endpoint_pct` (number): Percentage of endpoints with at least one executed test case.
- `categories` (object): Per-category breakdown (`positive`, `negative`, `boundary`, `security`) with `total`, `executed`, and `pct` fields.
- `response_codes` (object): Frequency distribution of HTTP status codes returned by the target API.
- `fields` (object): Field coverage with `total`, `covered`, and `pct` — percentage of schema-defined request fields covered by test case payloads.
- `generated_at` (string): ISO 8601 timestamp of when the report was computed.

### 5. Get Regression Report
Computes the diff between the specified schema and its predecessor in the same project.

```bash
curl http://localhost:8080/api/schemas/00000000-0000-0000-0000-000000000001/regression \
  -H "X-API-Key: your_user_api_key"
```

#### Response JSON:
- `base_schema_id` (string): UUID of the predecessor schema (null UUID if first upload).
- `target_schema_id` (string): UUID of the target schema.
- `added` (array): Endpoints present in target but absent from the predecessor.
- `removed` (array): Endpoints present in the predecessor but absent from the target.
- `modified` (array): Endpoints present in both but with changes. Each entry includes:
  - `method` (string): HTTP method.
  - `path` (string): Endpoint path.
  - `parameters_changed` (bool): Whether parameters differ.
  - `request_schema_changed` (bool): Whether the request schema differs.
  - `response_schema_changed` (bool): Whether the response schema differs.
  - `auth_changed` (bool): Whether the `AuthRequired` flag differs.
