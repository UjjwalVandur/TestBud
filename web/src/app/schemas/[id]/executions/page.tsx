"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { listExecutions, triggerExecution } from "@/lib/api";
import type { ExecutionListResult, ExecutionRunResult } from "@/lib/types";

export default function ExecutionsPage() {
  const params = useParams();
  const schemaId = params.id as string;
  const [data, setData] = useState<ExecutionListResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Trigger form state
  const [showTrigger, setShowTrigger] = useState(false);
  const [targetUrl, setTargetUrl] = useState("");
  const [authHeaders, setAuthHeaders] = useState("");
  const [altAuthHeaders, setAltAuthHeaders] = useState("");
  const [running, setRunning] = useState(false);
  const [runResult, setRunResult] = useState<ExecutionRunResult | null>(null);

  function refresh(isInitial = false) {
    if (!isInitial) setLoading(true);
    listExecutions(schemaId)
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    refresh(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [schemaId]);

  async function handleTrigger(e: React.FormEvent) {
    e.preventDefault();
    setRunning(true);
    setError("");
    setRunResult(null);
    try {
      const parseHeaders = (s: string) => {
        if (!s.trim()) return undefined;
        return JSON.parse(s) as Record<string, string>;
      };
      const result = await triggerExecution(
        schemaId,
        targetUrl,
        parseHeaders(authHeaders),
        parseHeaders(altAuthHeaders)
      );
      setRunResult(result);
      setShowTrigger(false);
      refresh();
    } catch (ex: unknown) {
      setError((ex as Error).message);
    } finally {
      setRunning(false);
    }
  }

  const summary = data?.summary;
  const passRate =
    summary && summary.total > 0
      ? ((summary.passed / summary.total) * 100).toFixed(1)
      : "0";

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
          / Executions
        </div>
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">Execution History</h1>
          <button onClick={() => setShowTrigger(true)} className="btn-primary">
            ▶ Run Tests
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-6 rounded-xl bg-[var(--accent-rose)]/10 border border-[var(--accent-rose)]/20 px-5 py-4 text-sm text-[var(--accent-rose)]">
          {error}
        </div>
      )}

      {runResult && (
        <div className="mb-6 rounded-xl bg-[var(--accent-emerald)]/10 border border-[var(--accent-emerald)]/20 px-5 py-4 text-sm text-[var(--accent-emerald)]">
          Execution complete — {runResult.passed} passed, {runResult.failed}{" "}
          failed out of {runResult.total} test cases.
        </div>
      )}

      {/* Trigger modal */}
      {showTrigger && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-card w-full max-w-lg p-8 animate-slide-up">
            <h2 className="text-xl font-bold mb-1">Trigger Test Execution</h2>
            <p className="text-sm text-[var(--text-secondary)] mb-6">
              Run all generated test cases against your target API.
            </p>
            <form onSubmit={handleTrigger} className="flex flex-col gap-4">
              <div>
                <label className="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]">
                  Target URL *
                </label>
                <input
                  required
                  value={targetUrl}
                  onChange={(e) => setTargetUrl(e.target.value)}
                  placeholder="https://your-api.example.com"
                  className="w-full"
                />
              </div>
              <div>
                <label className="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]">
                  Auth Headers (JSON, optional)
                </label>
                <textarea
                  value={authHeaders}
                  onChange={(e) => setAuthHeaders(e.target.value)}
                  placeholder={'{"Authorization": "Bearer ..."}'}
                  rows={2}
                  className="w-full font-mono text-xs"
                />
              </div>
              <div>
                <label className="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]">
                  Alt Auth Headers (JSON, optional)
                </label>
                <textarea
                  value={altAuthHeaders}
                  onChange={(e) => setAltAuthHeaders(e.target.value)}
                  placeholder={'{"Authorization": "Bearer alt_user_token"}'}
                  rows={2}
                  className="w-full font-mono text-xs"
                />
              </div>
              <div className="flex justify-end gap-3 mt-2">
                <button
                  type="button"
                  onClick={() => setShowTrigger(false)}
                  className="btn-secondary"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={running}
                  className="btn-primary"
                >
                  {running ? "Running…" : "Execute"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Summary cards */}
      {summary && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8">
          <div className="glass-card p-4 text-center">
            <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mb-1 font-medium">
              Total
            </p>
            <p className="text-xl font-bold">{summary.total}</p>
          </div>
          <div className="glass-card p-4 text-center">
            <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mb-1 font-medium">
              Passed
            </p>
            <p className="text-xl font-bold text-[var(--accent-emerald)]">
              {summary.passed}
            </p>
          </div>
          <div className="glass-card p-4 text-center">
            <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mb-1 font-medium">
              Failed
            </p>
            <p className="text-xl font-bold text-[var(--accent-rose)]">
              {summary.failed}
            </p>
          </div>
          <div className="glass-card p-4 text-center">
            <p className="text-xs uppercase tracking-wider text-[var(--text-muted)] mb-1 font-medium">
              Avg Response
            </p>
            <p className="text-xl font-bold">
              {summary.avg_response_ms.toFixed(0)}ms
            </p>
          </div>
        </div>
      )}

      {/* Pass rate bar */}
      {summary && summary.total > 0 && (
        <div className="glass-card p-6 mb-8">
          <div className="flex justify-between mb-2">
            <span className="text-sm font-medium">Pass Rate</span>
            <span className="text-sm font-bold">{passRate}%</span>
          </div>
          <div className="progress-bar">
            <div
              className="progress-fill"
              style={{
                width: `${passRate}%`,
                background:
                  Number(passRate) >= 80
                    ? "var(--accent-emerald)"
                    : Number(passRate) >= 50
                    ? "var(--accent-amber)"
                    : "var(--accent-rose)",
              }}
            />
          </div>
        </div>
      )}

      {/* Execution log */}
      {loading ? (
        <div className="flex items-center justify-center py-20 text-[var(--text-muted)]">
          Loading executions…
        </div>
      ) : !data || data.executions.length === 0 ? (
        <div className="glass-card flex flex-col items-center justify-center py-16 text-center">
          <span className="text-4xl mb-4">▶</span>
          <p className="text-[var(--text-secondary)]">
            No executions yet. Click &ldquo;Run Tests&rdquo; to get started.
          </p>
        </div>
      ) : (
        <div className="glass-card overflow-hidden">
          <div className="px-6 py-4 border-b border-[var(--border-subtle)]">
            <h2 className="text-base font-semibold">
              Execution Log ({data.executions.length})
            </h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-[var(--text-muted)] text-xs uppercase tracking-wider">
                  <th className="px-6 py-3 font-medium">Status</th>
                  <th className="px-6 py-3 font-medium">Category</th>
                  <th className="px-6 py-3 font-medium">Expected</th>
                  <th className="px-6 py-3 font-medium">Actual</th>
                  <th className="px-6 py-3 font-medium">Response</th>
                  <th className="px-6 py-3 font-medium">Time</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-subtle)]">
                {data.executions.map((ex) => (
                  <tr
                    key={ex.execution_id}
                    className="hover:bg-white/[0.02] transition-colors"
                  >
                    <td className="px-6 py-3">
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium ${
                          ex.passed ? "status-pass" : "status-fail"
                        }`}
                      >
                        {ex.passed ? "✓ Pass" : "✗ Fail"}
                      </span>
                    </td>
                    <td className="px-6 py-3">
                      <span
                        className={`cat-${ex.category} rounded-md px-2 py-0.5 text-xs font-medium`}
                      >
                        {ex.category}
                      </span>
                    </td>
                    <td className="px-6 py-3 font-mono text-xs">
                      {ex.expected_status}
                    </td>
                    <td className="px-6 py-3 font-mono text-xs">
                      {ex.actual_status}
                    </td>
                    <td className="px-6 py-3 text-[var(--text-secondary)]">
                      {ex.response_ms}ms
                    </td>
                    <td className="px-6 py-3 text-[var(--text-muted)] text-xs">
                      {new Date(ex.ran_at).toLocaleString()}
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
