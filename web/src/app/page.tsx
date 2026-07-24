"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { listSchemas } from "@/lib/api";
import type { SchemaListItem } from "@/lib/types";

export default function DashboardPage() {
  const [schemas, setSchemas] = useState<SchemaListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    listSchemas()
      .then(setSchemas)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  const totalEndpoints = schemas.reduce((s, sc) => s + sc.endpoint_count, 0);
  const projectIds = new Set(schemas.map((s) => s.project_id));

  return (
    <div className="animate-slide-up">
      {/* Hero */}
      <div className="mb-10">
        <h1 className="text-3xl font-bold mb-2">
          Welcome to <span className="gradient-text">TestBud</span>
        </h1>
        <p className="text-[var(--text-secondary)] text-base">
          Automated API test case generation, execution &amp; coverage analytics.
        </p>
      </div>

      {error && (
        <div className="mb-6 rounded-xl bg-[var(--accent-rose)]/10 border border-[var(--accent-rose)]/20 px-5 py-4 text-sm text-[var(--accent-rose)]">
          {error} — Make sure your API key is configured in Settings.
        </div>
      )}

      {/* KPI cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 mb-10">
        <KPICard
          label="Schemas Uploaded"
          value={loading ? "—" : schemas.length.toString()}
          accent="blue"
        />
        <KPICard
          label="Projects"
          value={loading ? "—" : projectIds.size.toString()}
          accent="violet"
        />
        <KPICard
          label="Total Endpoints"
          value={loading ? "—" : totalEndpoints.toString()}
          accent="emerald"
        />
        <KPICard
          label="Latest Version"
          value={loading || schemas.length === 0 ? "—" : schemas[0].version}
          accent="amber"
        />
      </div>

      {/* Quick actions */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-5 mb-10">
        <Link href="/schemas" className="no-underline">
          <div className="glass-card p-6 cursor-pointer group">
            <div className="flex items-center gap-3 mb-3">
              <span className="text-2xl">📋</span>
              <h3 className="text-base font-semibold text-[var(--text-primary)] group-hover:text-[var(--accent-blue)] transition-colors">
                Manage Schemas
              </h3>
            </div>
            <p className="text-sm text-[var(--text-secondary)]">
              Upload, browse, and inspect OpenAPI schemas with auto-generated test case breakdowns.
            </p>
          </div>
        </Link>

        <div className="glass-card p-6">
          <div className="flex items-center gap-3 mb-3">
            <span className="text-2xl">🚀</span>
            <h3 className="text-base font-semibold">Quick Start</h3>
          </div>
          <p className="text-sm text-[var(--text-secondary)] mb-3">
            1. Configure your API key in Settings<br/>
            2. Upload an OpenAPI schema<br/>
            3. Trigger test execution against your target API<br/>
            4. View coverage analytics &amp; regression diffs
          </p>
        </div>
      </div>

      {/* Recent schemas table */}
      {!loading && schemas.length > 0 && (
        <div className="glass-card overflow-hidden">
          <div className="px-6 py-4 border-b border-[var(--border-subtle)]">
            <h2 className="text-base font-semibold">Recent Schemas</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-[var(--text-muted)] text-xs uppercase tracking-wider">
                  <th className="px-6 py-3 font-medium">Version</th>
                  <th className="px-6 py-3 font-medium">OpenAPI</th>
                  <th className="px-6 py-3 font-medium">Endpoints</th>
                  <th className="px-6 py-3 font-medium">Uploaded</th>
                  <th className="px-6 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-subtle)]">
                {schemas.slice(0, 5).map((s) => (
                  <tr
                    key={s.schema_id}
                    className="hover:bg-white/[0.02] transition-colors"
                  >
                    <td className="px-6 py-4 font-medium">{s.version}</td>
                    <td className="px-6 py-4 text-[var(--text-secondary)]">
                      {s.openapi_version}
                    </td>
                    <td className="px-6 py-4">{s.endpoint_count}</td>
                    <td className="px-6 py-4 text-[var(--text-secondary)]">
                      {new Date(s.uploaded_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <Link
                        href={`/schemas/${s.schema_id}`}
                        className="text-[var(--accent-blue)] hover:underline text-xs font-medium no-underline"
                      >
                        View →
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function KPICard({
  label,
  value,
  accent,
}: {
  label: string;
  value: string;
  accent: "blue" | "violet" | "emerald" | "amber";
}) {
  const colors = {
    blue: "from-blue-500/20 to-blue-600/5 border-blue-500/20",
    violet: "from-violet-500/20 to-violet-600/5 border-violet-500/20",
    emerald: "from-emerald-500/20 to-emerald-600/5 border-emerald-500/20",
    amber: "from-amber-500/20 to-amber-600/5 border-amber-500/20",
  };

  return (
    <div
      className={`rounded-2xl border bg-gradient-to-br p-6 ${colors[accent]}`}
    >
      <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mb-2 font-medium">
        {label}
      </p>
      <p className="text-3xl font-bold">{value}</p>
    </div>
  );
}
