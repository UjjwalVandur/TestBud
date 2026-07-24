import type {
  SchemaListItem,
  SchemaDetailResult,
  UploadSchemaResult,
  ExecutionListResult,
  ExecutionRunResult,
  CoverageReportResult,
  RegressionReport,
} from "./types";

const BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

function getApiKey(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem("testbud_api_key") ?? "";
}

async function request<T>(
  path: string,
  opts: RequestInit = {}
): Promise<T> {
  const key = getApiKey();
  const headers: Record<string, string> = {
    ...(opts.headers as Record<string, string>),
  };
  if (key) headers["X-API-Key"] = key;

  const res = await fetch(`${BASE_URL}${path}`, { ...opts, headers });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(
      (body as { error?: string }).error ?? `HTTP ${res.status}`
    );
  }
  return res.json() as Promise<T>;
}

/* ── Health ── */
export async function checkHealth(): Promise<{ status: string }> {
  return request("/health");
}

/* ── Schemas ── */
export async function listSchemas(
  projectId?: string
): Promise<SchemaListItem[]> {
  const q = projectId ? `?project_id=${projectId}` : "";
  return request(`/api/schemas${q}`);
}

export async function getSchemaDetail(
  id: string
): Promise<SchemaDetailResult> {
  return request(`/api/schemas/${id}`);
}

export async function uploadSchema(
  projectId: string,
  version: string,
  file: File
): Promise<UploadSchemaResult> {
  const key = getApiKey();
  const form = new FormData();
  form.append("project_id", projectId);
  form.append("version", version);
  form.append("file", file);

  const headers: Record<string, string> = {};
  if (key) headers["X-API-Key"] = key;

  const res = await fetch(`${BASE_URL}/api/schemas`, {
    method: "POST",
    headers,
    body: form,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(
      (body as { error?: string }).error ?? `HTTP ${res.status}`
    );
  }
  return res.json() as Promise<UploadSchemaResult>;
}

/* ── Executions ── */
export async function triggerExecution(
  schemaId: string,
  targetUrl: string,
  authHeaders?: Record<string, string>,
  altAuthHeaders?: Record<string, string>
): Promise<ExecutionRunResult> {
  return request(`/api/schemas/${schemaId}/executions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      target_url: targetUrl,
      auth_headers: authHeaders,
      alt_auth_headers: altAuthHeaders,
    }),
  });
}

export async function listExecutions(
  schemaId: string
): Promise<ExecutionListResult> {
  return request(`/api/schemas/${schemaId}/executions`);
}

/* ── Coverage ── */
export async function getCoverage(
  schemaId: string
): Promise<CoverageReportResult> {
  return request(`/api/schemas/${schemaId}/coverage`);
}

/* ── Regression ── */
export async function getRegression(
  schemaId: string
): Promise<RegressionReport> {
  return request(`/api/schemas/${schemaId}/regression`);
}
