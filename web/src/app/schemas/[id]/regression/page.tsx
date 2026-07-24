"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { getRegression } from "@/lib/api";
import type { RegressionReport } from "@/lib/types";

export default function RegressionPage() {
  const params = useParams();
  const schemaId = params.id as string;
  const [report, setReport] = useState<RegressionReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    getRegression(schemaId)
      .then(setReport)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [schemaId]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-[var(--text-muted)]">
        Computing regression diff…
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

  const isNilUUID =
    report.base_schema_id === "00000000-0000-0000-0000-000000000000";
  const totalChanges =
    report.added.length + report.removed.length + report.modified.length;

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
          / Regression
        </div>
        <h1 className="text-2xl font-bold mb-1">Regression Report</h1>
        <p className="text-sm text-[var(--text-secondary)]">
          {isNilUUID
            ? "First schema version — no predecessor to diff against."
            : `Comparing against predecessor ${report.base_schema_id.slice(0, 8)}…`}
        </p>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="glass-card p-5 text-center border-l-4 border-l-[var(--accent-emerald)]">
          <p className="text-3xl font-bold text-[var(--accent-emerald)]">
            {report.added.length}
          </p>
          <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mt-1 font-medium">
            Added
          </p>
        </div>
        <div className="glass-card p-5 text-center border-l-4 border-l-[var(--accent-rose)]">
          <p className="text-3xl font-bold text-[var(--accent-rose)]">
            {report.removed.length}
          </p>
          <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mt-1 font-medium">
            Removed
          </p>
        </div>
        <div className="glass-card p-5 text-center border-l-4 border-l-[var(--accent-amber)]">
          <p className="text-3xl font-bold text-[var(--accent-amber)]">
            {report.modified.length}
          </p>
          <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mt-1 font-medium">
            Modified
          </p>
        </div>
      </div>

      {totalChanges === 0 && (
        <div className="glass-card flex flex-col items-center justify-center py-16 text-center">
          <span className="text-4xl mb-4">✅</span>
          <p className="text-[var(--text-secondary)]">
            No changes detected between schema versions.
          </p>
        </div>
      )}

      {/* Added endpoints */}
      {report.added.length > 0 && (
        <section className="mb-8">
          <h2 className="text-base font-semibold mb-3 flex items-center gap-2">
            <span className="inline-block h-3 w-3 rounded-sm bg-[var(--accent-emerald)]" />
            Added Endpoints
          </h2>
          <div className="glass-card overflow-hidden divide-y divide-[var(--border-subtle)]">
            {report.added.map((ep, i) => (
              <div key={i} className="px-6 py-3 flex items-center gap-3">
                <span
                  className={`method-badge method-${ep.method.toLowerCase()}`}
                >
                  {ep.method}
                </span>
                <span className="font-mono text-sm">{ep.path}</span>
                <span className="ml-auto text-xs font-medium text-[var(--accent-emerald)]">
                  + NEW
                </span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Removed endpoints */}
      {report.removed.length > 0 && (
        <section className="mb-8">
          <h2 className="text-base font-semibold mb-3 flex items-center gap-2">
            <span className="inline-block h-3 w-3 rounded-sm bg-[var(--accent-rose)]" />
            Removed Endpoints
          </h2>
          <div className="glass-card overflow-hidden divide-y divide-[var(--border-subtle)]">
            {report.removed.map((ep, i) => (
              <div key={i} className="px-6 py-3 flex items-center gap-3">
                <span
                  className={`method-badge method-${ep.method.toLowerCase()}`}
                >
                  {ep.method}
                </span>
                <span className="font-mono text-sm line-through text-[var(--text-muted)]">
                  {ep.path}
                </span>
                <span className="ml-auto text-xs font-medium text-[var(--accent-rose)]">
                  − REMOVED
                </span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Modified endpoints */}
      {report.modified.length > 0 && (
        <section className="mb-8">
          <h2 className="text-base font-semibold mb-3 flex items-center gap-2">
            <span className="inline-block h-3 w-3 rounded-sm bg-[var(--accent-amber)]" />
            Modified Endpoints
          </h2>
          <div className="glass-card overflow-hidden divide-y divide-[var(--border-subtle)]">
            {report.modified.map((ep, i) => (
              <div key={i} className="px-6 py-4">
                <div className="flex items-center gap-3 mb-3">
                  <span
                    className={`method-badge method-${ep.method.toLowerCase()}`}
                  >
                    {ep.method}
                  </span>
                  <span className="font-mono text-sm">{ep.path}</span>
                  <span className="ml-auto text-xs font-medium text-[var(--accent-amber)]">
                    ~ MODIFIED
                  </span>
                </div>
                <div className="flex flex-wrap gap-2">
                  {ep.parameters_changed && (
                    <ChangeTag label="Parameters" />
                  )}
                  {ep.request_schema_changed && (
                    <ChangeTag label="Request Schema" />
                  )}
                  {ep.response_schema_changed && (
                    <ChangeTag label="Response Schema" />
                  )}
                  {ep.auth_changed && (
                    <ChangeTag label="Auth Required" />
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function ChangeTag({ label }: { label: string }) {
  return (
    <span className="rounded-md bg-[var(--accent-amber)]/10 border border-[var(--accent-amber)]/20 px-2.5 py-1 text-xs font-medium text-[var(--accent-amber)]">
      {label}
    </span>
  );
}
