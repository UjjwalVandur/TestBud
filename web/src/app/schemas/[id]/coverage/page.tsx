"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { getCoverage } from "@/lib/api";
import type { CoverageReportResult } from "@/lib/types";

export default function CoveragePage() {
  const params = useParams();
  const schemaId = params.id as string;
  const [report, setReport] = useState<CoverageReportResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    getCoverage(schemaId)
      .then(setReport)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [schemaId]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-[var(--text-muted)]">
        Computing coverage…
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
  if (!report) return null;

  const categories = Object.entries(report.categories);
  const responseCodes = Object.entries(report.response_codes).sort(
    ([a], [b]) => Number(a) - Number(b)
  );
  const totalResponses = responseCodes.reduce(
    (s, [, v]) => s + v,
    0
  );

  return (
    <div className="animate-slide-up">
      {/* Breadcrumb + header */}
      <div className="mb-8">
        <div className="text-xs text-[var(--text-muted)] mb-2">
          <Link href="/schemas" className="hover:text-[var(--accent-blue)] no-underline">
            Schemas
          </Link>{" "}
          /{" "}
          <Link
            href={`/schemas/${schemaId}`}
            className="hover:text-[var(--accent-blue)] no-underline"
          >
            {schemaId.slice(0, 8)}…
          </Link>{" "}
          / Coverage
        </div>
        <h1 className="text-2xl font-bold mb-1">Coverage Analytics</h1>
        <p className="text-sm text-[var(--text-secondary)]">
          Generated{" "}
          {new Date(report.generated_at).toLocaleString()}
        </p>
      </div>

      {/* Main endpoint coverage */}
      <div className="glass-card p-8 mb-8 text-center">
        <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mb-3 font-medium">
          Endpoint Coverage
        </p>
        <p className="text-5xl font-bold gradient-text mb-4">
          {report.endpoint_pct.toFixed(1)}%
        </p>
        <div className="progress-bar mx-auto max-w-md">
          <div
            className="progress-fill"
            style={{
              width: `${report.endpoint_pct}%`,
              background:
                report.endpoint_pct >= 80
                  ? "var(--accent-emerald)"
                  : report.endpoint_pct >= 50
                  ? "var(--accent-amber)"
                  : "var(--accent-rose)",
            }}
          />
        </div>
      </div>

      {/* Category coverage grid */}
      <h2 className="text-base font-semibold mb-4">Category Coverage</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {categories.map(([name, stats]) => (
          <div key={name} className="glass-card p-5">
            <div className="flex items-center justify-between mb-3">
              <span
                className={`cat-${name} rounded-md px-2.5 py-1 text-xs font-semibold capitalize`}
              >
                {name}
              </span>
              <span className="text-lg font-bold">{stats.pct.toFixed(0)}%</span>
            </div>
            <div className="progress-bar mb-2">
              <div
                className="progress-fill"
                style={{
                  width: `${stats.pct}%`,
                  background:
                    name === "positive"
                      ? "var(--accent-emerald)"
                      : name === "negative"
                      ? "var(--accent-rose)"
                      : name === "boundary"
                      ? "var(--accent-amber)"
                      : "var(--accent-violet)",
                }}
              />
            </div>
            <p className="text-xs text-[var(--text-muted)]">
              {stats.executed} / {stats.total} executed
            </p>
          </div>
        ))}
      </div>

      {/* Response code distribution + Field coverage */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Response codes */}
        <div className="glass-card p-6">
          <h2 className="text-base font-semibold mb-4">
            Response Code Distribution
          </h2>
          {responseCodes.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">No data yet.</p>
          ) : (
            <div className="space-y-3">
              {responseCodes.map(([code, count]) => {
                const pct = totalResponses > 0 ? (count / totalResponses) * 100 : 0;
                const codeNum = Number(code);
                const color =
                  codeNum < 300
                    ? "var(--accent-emerald)"
                    : codeNum < 500
                    ? "var(--accent-amber)"
                    : "var(--accent-rose)";
                return (
                  <div key={code}>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="font-mono font-medium">{code}</span>
                      <span className="text-[var(--text-muted)]">
                        {count} ({pct.toFixed(0)}%)
                      </span>
                    </div>
                    <div className="progress-bar">
                      <div
                        className="progress-fill"
                        style={{ width: `${pct}%`, background: color }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Field coverage */}
        <div className="glass-card p-6">
          <h2 className="text-base font-semibold mb-4">Field Coverage</h2>
          <div className="text-center py-6">
            <p className="text-4xl font-bold gradient-text mb-2">
              {report.fields.pct.toFixed(1)}%
            </p>
            <p className="text-sm text-[var(--text-muted)]">
              {report.fields.covered} of {report.fields.total} schema-defined
              fields covered
            </p>
          </div>
          <div className="progress-bar">
            <div
              className="progress-fill"
              style={{
                width: `${report.fields.pct}%`,
                background:
                  report.fields.pct >= 80
                    ? "var(--accent-emerald)"
                    : report.fields.pct >= 50
                    ? "var(--accent-amber)"
                    : "var(--accent-rose)",
              }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
