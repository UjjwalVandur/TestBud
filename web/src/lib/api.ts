import type {
  SchemaListItem,
  SchemaDetailResult,
  UploadSchemaResult,
  ExecutionListResult,
  ExecutionRunResult,
  CoverageReportResult,
  RegressionReport,
  TestCaseDetail,
} from "./types";

const BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function getToken(): Promise<string> {
  if (typeof window === "undefined") return "";
  // @ts-expect-error window.Clerk is injected by Next.js ClerkProvider
  if (window.Clerk?.session) {
    // @ts-expect-error window.Clerk is injected by Next.js ClerkProvider
    return await window.Clerk.session.getToken();
  }
  // Fallback for CLI/local testing
  return localStorage.getItem("testbud_api_key") ?? "";
}

async function request<T>(
  path: string,
  opts: RequestInit = {}
): Promise<T> {
  const token = await getToken();
  const headers: Record<string, string> = {
    ...(opts.headers as Record<string, string>),
  };
  
  if (token) {
    if (token.startsWith("tb_")) {
      headers["X-API-Key"] = token;
    } else {
      headers["Authorization"] = `Bearer ${token}`;
    }
  }

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
  const form = new FormData();
  form.append("project_id", projectId);
  form.append("version", version);
  form.append("file", file);

  return request("/api/schemas", {
    method: "POST",
    body: form,
  });
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

export async function getTestCasesByEndpoint(
  endpointId: string
): Promise<TestCaseDetail[]> {
  return request(`/api/endpoints/${endpointId}/testcases`);
}
