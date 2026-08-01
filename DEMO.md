# TestBud — 5-Minute Demo Guide

This guide walks you through every feature of TestBud using the included Docker environment and sample Petstore API.

---

## Prerequisites

- Docker & Docker Compose installed
- The TestBud repository cloned locally

---

## Step 1: Start Everything

```bash
docker compose up -d
```

This starts 4 services:

| Service | URL | Purpose |
|---|---|---|
| `db` | `localhost:5432` | PostgreSQL database |
| `backend` | `http://localhost:8080` | TestBud Go API |
| `frontend` | `http://localhost:3000` | TestBud Dashboard |
| `mock-target` | `http://localhost:8090` | Sample Petstore API |

Wait ~30 seconds for all services to become healthy. You can check with:

```bash
docker compose ps
```

A demo user is automatically created on startup:
- **API Key:** `testbud-demo-key-2026`

---

## Step 2: Configure the Dashboard

1. Open **http://localhost:3000** in your browser.
2. Click the **⚙ Settings** icon in the top-right corner.
3. Enter the API key: `testbud-demo-key-2026`
4. Click **Save**.

---

## Step 3: Upload Your First Schema

1. Navigate to the **Schemas** page.
2. Click **+ Upload Schema**.
3. Fill in:
   - **Project ID:** `11111111-1111-1111-1111-111111111111`
   - **Version:** `1.0.0`
4. Drag and drop `examples/petstore-v1.yaml` into the file dropzone.
5. Click **Upload**.

You'll see the schema appear with 4 endpoints. Click into it to see:
- HTTP method badges (GET, POST, DELETE)
- Auth indicators on protected routes
- Test case counts by category (positive, negative, boundary, security)

---

## Step 4: Execute Tests

1. From the schema detail page, click **▶ Executions**.
2. Click **▶ Run Tests**.
3. Enter Target URL: `http://mock-target:8090`
4. Click **Execute**.

The concurrent execution engine fires all generated test cases against the mock server. You'll see:
- **Pass rate progress bar** (expect ~70-85% — some negative/boundary tests will correctly fail)
- **Execution log table** with pass/fail status, expected vs actual status codes, and response times

---

## Step 5: View Coverage Analytics

1. Click **📊 Coverage** from the schema detail page.
2. You'll see:
   - **Endpoint Coverage %** — how many endpoints have been tested
   - **Category Coverage** — breakdown by positive/negative/boundary/security
   - **Response Code Distribution** — bar chart of 200s, 400s, 401s, etc.
   - **Field Coverage** — how many schema-defined fields were covered

---

## Step 6: Regression Detection

1. Go back to **Schemas** and upload a second schema:
   - **Project ID:** `11111111-1111-1111-1111-111111111111` (same project!)
   - **Version:** `2.0.0`
   - **File:** `examples/petstore-v2.yaml`
2. Open the new schema and click **🔀 Regression**.

You'll see the diff between v1 and v2:
- 🟢 **Added:** `PUT /pets/{petId}` (new endpoint)
- 🔴 **Removed:** `DELETE /pets/{petId}` (no longer in spec)
- 🟡 **Modified:** `POST /pets` (request schema changed — `age` field added)

---

## Step 7: CI/CD CLI

Run the CLI from your terminal (outside Docker):

```bash
go run ./cmd/cli \
  --api-url http://localhost:8080 \
  --api-key testbud-demo-key-2026 \
  --project-id 22222222-2222-2222-2222-222222222222 \
  --version "cli-test" \
  --schema-file examples/petstore-v1.yaml \
  --target-url http://localhost:8090 \
  --fail-threshold 70
```

You'll see formatted output with:
- Upload confirmation
- Test execution summary
- Failed tests table with category, expected vs actual status
- Exit code based on pass rate vs threshold

---

## Step 8: Cleanup

```bash
docker compose down -v
```

This stops all containers and removes the database volume.

---

## Feature Summary

| Feature | Where to See It |
|---|---|
| Schema Parsing | Upload → endpoint list with parameters |
| Test Generation | Schema detail → test case counts by category |
| Concurrent Execution | Executions → run tests → results in seconds |
| Coverage Analytics | Coverage page → endpoint %, categories, response codes |
| Regression Detection | Upload v2 → regression page → added/removed/modified |
| CI/CD Integration | CLI tool with exit codes for pipeline gating |
| Dashboard UI | Dark theme, glassmorphism, micro-animations |
