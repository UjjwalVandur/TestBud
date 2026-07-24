/* ── TypeScript interfaces mirroring Go backend DTOs ── */

export interface SchemaListItem {
  schema_id: string;
  project_id: string;
  version: string;
  openapi_version: string;
  endpoint_count: number;
  uploaded_at: string;
}

export interface EndpointDetail {
  endpoint_id: string;
  method: string;
  path: string;
  auth_required: boolean;
  test_counts: Record<string, number>;
  total_tests: number;
}

export interface SchemaDetailResult {
  schema_id: string;
  project_id: string;
  version: string;
  openapi_version: string;
  uploaded_at: string;
  endpoints: EndpointDetail[];
  total_endpoints: number;
  total_test_cases: number;
}

export interface UploadSchemaResult {
  schema_id: string;
  schema_hash: string;
  openapi_version: string;
  endpoint_count: number;
}

export interface ExecutionItem {
  execution_id: string;
  test_case_id: string;
  category: string;
  expected_status: number;
  actual_status: number;
  response_ms: number;
  passed: boolean;
  ran_at: string;
}

export interface ExecutionSummary {
  total: number;
  passed: number;
  failed: number;
  avg_response_ms: number;
}

export interface ExecutionListResult {
  schema_id: string;
  summary: ExecutionSummary;
  executions: ExecutionItem[];
}

export interface ExecutionRunResult {
  schema_id: string;
  total: number;
  passed: number;
  failed: number;
}

export interface CategoryStats {
  total: number;
  executed: number;
  pct: number;
}

export interface FieldStats {
  total: number;
  covered: number;
  pct: number;
}

export interface CoverageReportResult {
  schema_id: string;
  endpoint_pct: number;
  categories: Record<string, CategoryStats>;
  response_codes: Record<string, number>;
  fields: FieldStats;
  generated_at: string;
}

export interface RegressionEndpoint {
  method: string;
  path: string;
}

export interface ModifiedEndpoint {
  method: string;
  path: string;
  parameters_changed: boolean;
  request_schema_changed: boolean;
  response_schema_changed: boolean;
  auth_changed: boolean;
}

export interface RegressionReport {
  base_schema_id: string;
  target_schema_id: string;
  added: RegressionEndpoint[];
  removed: RegressionEndpoint[];
  modified: ModifiedEndpoint[];
}
