# TestBud

Automated API Test Case Generator & Execution Platform.

TestBud is a portfolio-grade, production-style platform built entirely on free-tier infrastructure. It parses OpenAPI 3.x / Swagger 2.x schemas, automatically generates positive, negative, boundary, and security test cases, and executes them concurrently against a target live API.

The repository is at **v1.0.0** — all 8 weeks of the roadmap are complete.

---

## Architecture & Core Modules

1. **Schema Parser**: Validates and normalizes **OpenAPI/Swagger** schemas (via `kin-openapi`) and **Bruno API Client files** (`.bru` or `.zip` collections) with an intelligent schema inferencer.
2. **Test Generator**: A rule-based, deterministic engine generating positive, negative, boundary, and security test cases per endpoint.
3. **Concurrent Execution Engine**: A worker pool capped at **10 concurrent workers** (Render RAM memory safety ceiling) that pulls test cases and runs them via HTTP client (10s timeout) with full `context.Context` cancellation.
4. **Deduplication Engine**: Endpoint-level hash matching (`endpoint_hash`). Test cases are regenerated only for new or modified endpoints, minimizing free-tier compute usage.
5. **Execution Retention Policy**: Daily cron job running at 2:00 AM using `robfig/cron/v3` to clean up execution logs older than 90 days, ensuring Neon's 512MB storage cap is never exceeded.
6. **Coverage Analyzer**: Computes endpoint coverage %, category coverage breakdown, response code distribution, and field coverage for each schema version.
7. **Regression Detector**: Diffs parsed internal representations between schema versions; flags added, removed, and modified endpoints (parameters, request/response schemas, auth changes). When only `AuthRequired` changes, non-security test cases are preserved and only security cases are regenerated.
8. **Dashboard APIs**: REST endpoints for schema listing, detail views with test case breakdowns, and execution history with summary statistics.
9. **Frontend Dashboard**: Next.js 14 + Tailwind CSS single-page application with schema management, execution control, coverage analytics, and regression diff visualization.
10. **CI/CD CLI**: Standalone Go binary (`cmd/cli`) that automates upload → execute → report in a single command, with configurable pass rate thresholds and structured exit codes for pipeline gating.
11. **AI Test Generator**: Optional LLM-powered test generation module (`internal/aigenerator`) using Gemma 4 via AWS Bedrock. Uses a `CompositeGenerator` pattern to supplement rule-based tests with AI-generated business logic edge cases, featuring automated rate-limiting and graceful degradation.

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

## Docker Quickstart

The fastest way to run the full platform (database + backend + frontend + mock API):

```bash
docker compose up -d
```

This starts 4 services:

| Service | URL | Purpose |
|---|---|---|
| `db` | `localhost:5432` | PostgreSQL 16 |
| `backend` | `http://localhost:8080` | Go API server |
| `frontend` | `http://localhost:3000` | Next.js dashboard |
| `mock-target` | `http://localhost:8090` | Sample Petstore API |

A demo user is automatically seeded on startup:
- **API Key:** `testbud-demo-key-2026`

For a full walkthrough of every feature, see [DEMO.md](DEMO.md).

To tear down:

```bash
docker compose down -v
```

---

## Running the Platform (Manual)

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

### 6. List Schemas
Returns all schemas uploaded by the authenticated user.

```bash
curl http://localhost:8080/api/schemas \
  -H "X-API-Key: your_user_api_key"
```

Optional query parameter: `?project_id=<UUID>` to filter by project.

### 7. Get Schema Detail
Returns full schema details with endpoints and per-endpoint test case counts.

```bash
curl http://localhost:8080/api/schemas/00000000-0000-0000-0000-000000000001 \
  -H "X-API-Key: your_user_api_key"
```

### 8. List Executions
Returns execution history with summary statistics for a schema.

```bash
curl http://localhost:8080/api/schemas/00000000-0000-0000-0000-000000000001/executions \
  -H "X-API-Key: your_user_api_key"
```

---

## Frontend Dashboard

The frontend is a Next.js 14 application in the `web/` directory.

### Running the Frontend

```bash
cd web
npm install
npm run dev
```

The dashboard runs at `http://localhost:3000` and communicates with the Go backend at `http://localhost:8080`.

Configure your API key via the Settings modal in the top-right corner of the dashboard.

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Backend API base URL |

---

## CI/CD Integration

TestBud includes a standalone CLI binary for pipeline automation.

### Build the CLI

```bash
go build -o testbud-cli ./cmd/cli
```

### Usage

```bash
testbud-cli \
  --api-url https://testbud.example.com \
  --api-key YOUR_API_KEY \
  --project-id YOUR_PROJECT_UUID \
  --version "1.0.0" \
  --schema-file ./openapi.yaml \
  --target-url https://staging-api.example.com \
  --auth-headers '{"Authorization": "Bearer token"}' \
  --fail-threshold 100
```

### Flags

| Flag | Required | Description |
|---|---|---|
| `--api-url` | Yes | TestBud backend URL |
| `--api-key` | Yes | User API key |
| `--project-id` | Yes | Project UUID |
| `--version` | Yes | Schema version string |
| `--schema-file` | Yes | Path to OpenAPI/Swagger file |
| `--target-url` | Yes | Target API base URL |
| `--auth-headers` | No | JSON string of auth headers |
| `--alt-auth-headers` | No | JSON string of alt auth headers |
| `--fail-threshold` | No | Minimum pass rate % (default: 100) |

### Exit Codes

| Code | Meaning |
|---|---|
| `0` | Pass rate ≥ threshold |
| `1` | Pass rate < threshold (blocks pipeline) |
| `2` | Infrastructure error (bad flags, network, etc.) |

### GitHub Actions Example

See [`.github/workflows/testbud.yml`](.github/workflows/testbud.yml) for a drop-in workflow template.

---

## Demo

TestBud ships with everything needed for a self-contained demo:

- **Sample schemas:** `examples/petstore-v1.yaml` and `examples/petstore-v2.yaml`
- **Mock target API:** `examples/mock-server/` (Go stdlib HTTP server)
- **Step-by-step guide:** [DEMO.md](DEMO.md)
