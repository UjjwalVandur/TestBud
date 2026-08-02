"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { getSchemaDetail } from "@/lib/api";
import type { SchemaDetailResult } from "@/lib/types";

export default function SchemaDetailPage() {
  const params = useParams();
  const id = params.id as string;
  const [schema, setSchema] = useState<SchemaDetailResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    getSchemaDetail(id)
      .then(setSchema)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-[var(--text-muted)]">
        Loading schema details…
      </div>
    );
  }
  if (error) {
    return (
      <div className="rounded-xl bg-[var(--accent-rose)]/10 border border-[var(--accent-rose)]/20 px-5 py-4 text-sm text-[var(--accent-rose)]">
        {error}
      </div>
    );
  }
  if (!schema) return null;

  const categories = ["positive", "negative", "boundary", "security"] as const;

  return (
    <div className="animate-slide-up">
      {/* Breadcrumb + header */}
      <div className="mb-8">
        <div className="text-xs text-[var(--text-muted)] mb-2">
          <Link href="/schemas" className="hover:text-[var(--accent-blue)] no-underline">
            Schemas
          </Link>{" "}
          / v{schema.version}
        </div>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold mb-1">v{schema.version}</h1>
            <p className="text-sm text-[var(--text-secondary)]">
              OpenAPI {schema.openapi_version} · Uploaded{" "}
              {new Date(schema.uploaded_at).toLocaleDateString()}
            </p>
          </div>
          <div className="flex gap-3">
            <Link href={`/schemas/${id}/executions`} className="btn-secondary no-underline">
              ▶ Executions
            </Link>
            <Link href={`/schemas/${id}/coverage`} className="btn-secondary no-underline">
              📊 Coverage
            </Link>
            <Link href={`/schemas/${id}/regression`} className="btn-secondary no-underline">
              🔀 Regression
            </Link>
          </div>
        </div>
      </div>

      {/* Summary stats */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8">
        <StatCard label="Endpoints" value={schema.total_endpoints} />
        <StatCard label="Test Cases" value={schema.total_test_cases} />
        <StatCard label="Project" value={schema.project_id.slice(0, 8) + "…"} />
        <StatCard label="Schema ID" value={schema.schema_id.slice(0, 8) + "…"} />
      </div>

      {/* Endpoints list */}
      <div className="glass-card overflow-hidden">
        <div className="px-6 py-4 border-b border-[var(--border-subtle)]">
          <h2 className="text-base font-semibold">
            Endpoints ({schema.total_endpoints})
          </h2>
        </div>
        <div className="divide-y divide-[var(--border-subtle)]">
          {schema.endpoints.map((ep) => (
            <EndpointRow key={ep.endpoint_id} ep={ep} categories={categories} />
          ))}
        </div>
      </div>
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="glass-card p-4 text-center">
      <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mb-1 font-medium">
        {label}
      </p>
      <p className="text-xl font-bold">{value}</p>
    </div>
  );
}

import { getTestCasesByEndpoint } from "@/lib/api";
import type { TestCaseDetail, EndpointDetail } from "@/lib/types";

function EndpointRow({ ep, categories }: { ep: EndpointDetail, categories: readonly string[] }) {
  const [expanded, setExpanded] = useState(false);
  const [testCases, setTestCases] = useState<TestCaseDetail[] | null>(null);
  const [loading, setLoading] = useState(false);

  const toggleExpand = async () => {
    if (!expanded && !testCases) {
      setLoading(true);
      try {
        const data = await getTestCasesByEndpoint(ep.endpoint_id);
        setTestCases(data);
      } catch (err) {
        console.error("Failed to load test cases", err);
      } finally {
        setLoading(false);
      }
    }
    setExpanded(!expanded);
  };

  return (
    <div className="flex flex-col">
      <div
        className="px-6 py-4 hover:bg-white/[0.02] transition-colors cursor-pointer"
        onClick={toggleExpand}
      >
        <div className="flex items-center gap-3 mb-3">
          <span className={`method-badge method-${ep.method.toLowerCase()}`}>
            {ep.method}
          </span>
          <span className="font-mono text-sm">{ep.path}</span>
          {ep.auth_required && (
            <span className="ml-auto rounded-md bg-[var(--accent-amber)]/10 px-2 py-0.5 text-[10px] font-bold text-[var(--accent-amber)] uppercase tracking-wider">
              Auth
            </span>
          )}
        </div>

        {/* Test case category breakdown */}
        <div className="flex flex-wrap gap-2">
          {categories.map((cat) => {
            const count = ep.test_counts[cat] ?? 0;
            return (
              <span
                key={cat}
                className={`cat-${cat} rounded-md px-2.5 py-1 text-xs font-medium`}
              >
                {cat}: {count}
              </span>
            );
          })}
          <span className="ml-auto text-xs text-[var(--text-muted)]">
            {ep.total_tests} total {expanded ? "▲" : "▼"}
          </span>
        </div>
      </div>

      {expanded && (
        <div className="bg-black/20 border-t border-[var(--border-subtle)] p-6 overflow-hidden">
          {loading ? (
            <div className="text-sm text-[var(--text-muted)] animate-pulse">Loading test cases...</div>
          ) : testCases && testCases.length > 0 ? (
            <div className="flex flex-col gap-4">
              {testCases.map((tc) => {
                // Extract description and clean internal flags from the payload
                const rawPayload = tc.payload_json as Record<string, any>;
                const payload = { ...(rawPayload || {}) };
                const description = payload.description || "No description provided";
                
                // Delete internal execution flags to keep the UI clean
                delete payload.description;
                delete payload.omit_auth;
                delete payload.use_other_user_auth;
                delete payload.is_rate_limit_probe;
                delete payload.is_oversized_probe;
                delete payload.oversized_bytes;

                return (
                  <div key={tc.id} className="glass-card p-4 rounded-lg">
                    <div className="flex items-center justify-between mb-3 border-b border-[var(--border-subtle)] pb-3">
                      <div className="flex items-center gap-3">
                        <span className={`cat-${tc.category} rounded px-2 py-0.5 text-xs font-bold uppercase`}>
                          {tc.category.replace("_", " ")}
                        </span>
                        <span className="text-sm text-[var(--text-secondary)] font-medium">
                          {description}
                        </span>
                      </div>
                      <div className="text-sm text-[var(--text-secondary)] bg-black/30 px-2 py-1 rounded">
                        Expects: <span className="font-mono text-white font-bold">{tc.expected_status}</span>
                      </div>
                    </div>
                    <pre className="text-xs text-[var(--text-muted)] overflow-x-auto p-3 bg-black/40 rounded border border-white/5 font-mono">
                      {JSON.stringify(payload, null, 2)}
                    </pre>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="text-sm text-[var(--text-muted)]">No test cases found.</div>
          )}
        </div>
      )}
    </div>
  );
}
