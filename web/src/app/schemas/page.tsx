"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { listSchemas, uploadSchema } from "@/lib/api";
import type { SchemaListItem } from "@/lib/types";

export default function SchemasPage() {
  const [schemas, setSchemas] = useState<SchemaListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showUpload, setShowUpload] = useState(false);

  const refresh = useCallback(() => {
    setLoading(true);
    listSchemas()
      .then(setSchemas)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="animate-slide-up">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold mb-1">Schemas</h1>
          <p className="text-sm text-[var(--text-secondary)]">
            Upload and manage your OpenAPI / Swagger schemas.
          </p>
        </div>
        <button onClick={() => setShowUpload(true)} className="btn-primary">
          + Upload Schema
        </button>
      </div>

      {error && (
        <div className="mb-6 rounded-xl bg-[var(--accent-rose)]/10 border border-[var(--accent-rose)]/20 px-5 py-4 text-sm text-[var(--accent-rose)]">
          {error}
        </div>
      )}

      {/* Upload modal */}
      {showUpload && (
        <UploadModal
          onClose={() => setShowUpload(false)}
          onSuccess={() => {
            setShowUpload(false);
            refresh();
          }}
        />
      )}

      {/* Schema list */}
      {loading ? (
        <div className="flex items-center justify-center py-20 text-[var(--text-muted)]">
          Loading schemas…
        </div>
      ) : schemas.length === 0 ? (
        <div className="glass-card flex flex-col items-center justify-center py-20 text-center">
          <span className="text-4xl mb-4">📄</span>
          <p className="text-[var(--text-secondary)] mb-4">
            No schemas uploaded yet.
          </p>
          <button onClick={() => setShowUpload(true)} className="btn-primary">
            Upload Your First Schema
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {schemas.map((s) => (
            <Link
              key={s.schema_id}
              href={`/schemas/${s.schema_id}`}
              className="no-underline"
            >
              <div className="glass-card p-6 cursor-pointer group h-full flex flex-col">
                <div className="flex items-start justify-between mb-4">
                  <div>
                    <p className="text-lg font-semibold group-hover:text-[var(--accent-blue)] transition-colors">
                      v{s.version}
                    </p>
                    <p className="text-xs text-[var(--text-muted)] mt-0.5">
                      OpenAPI {s.openapi_version}
                    </p>
                  </div>
                  <span className="rounded-lg bg-[var(--accent-blue)]/10 px-3 py-1 text-xs font-bold text-[var(--accent-blue)]">
                    {s.endpoint_count} endpoints
                  </span>
                </div>

                <div className="mt-auto pt-4 border-t border-[var(--border-subtle)] flex items-center justify-between">
                  <span className="text-xs text-[var(--text-muted)]">
                    {new Date(s.uploaded_at).toLocaleDateString(undefined, {
                      month: "short",
                      day: "numeric",
                      year: "numeric",
                    })}
                  </span>
                  <span className="text-xs font-mono text-[var(--text-muted)]">
                    {s.schema_id.slice(0, 8)}…
                  </span>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

/* ── Upload modal ── */
function UploadModal({
  onClose,
  onSuccess,
}: {
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [projectId, setProjectId] = useState("");
  const [version, setVersion] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [err, setErr] = useState("");
  const [dragOver, setDragOver] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    setUploading(true);
    setErr("");
    try {
      await uploadSchema(projectId, version, file);
      onSuccess();
    } catch (ex: unknown) {
      setErr((ex as Error).message);
    } finally {
      setUploading(false);
    }
  }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="glass-card w-full max-w-lg p-8 animate-slide-up">
        <h2 className="text-xl font-bold mb-1">Upload Schema</h2>
        <p className="text-sm text-[var(--text-secondary)] mb-6">
          Upload an OpenAPI 3.x or Swagger 2.x schema file (JSON or YAML).
        </p>

        {err && (
          <div className="mb-4 rounded-lg bg-[var(--accent-rose)]/10 px-4 py-3 text-sm text-[var(--accent-rose)]">
            {err}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]">
              Project ID
            </label>
            <input
              required
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
              placeholder="UUID of the project"
              className="w-full"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]">
              Version
            </label>
            <input
              required
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="e.g. 1.0.0"
              className="w-full"
            />
          </div>

          {/* Dropzone */}
          <div
            onDragOver={(e) => {
              e.preventDefault();
              setDragOver(true);
            }}
            onDragLeave={() => setDragOver(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragOver(false);
              if (e.dataTransfer.files[0]) setFile(e.dataTransfer.files[0]);
            }}
            className={`flex flex-col items-center justify-center rounded-xl border-2 border-dashed p-8 text-center transition-colors cursor-pointer ${
              dragOver
                ? "border-[var(--accent-blue)] bg-[var(--accent-blue)]/5"
                : "border-[var(--border-subtle)] hover:border-[var(--text-muted)]"
            }`}
            onClick={() => document.getElementById("file-input")?.click()}
          >
            <input
              id="file-input"
              type="file"
              accept=".json,.yaml,.yml"
              className="hidden"
              onChange={(e) => {
                if (e.target.files?.[0]) setFile(e.target.files[0]);
              }}
            />
            {file ? (
              <p className="text-sm font-medium text-[var(--accent-blue)]">
                {file.name}{" "}
                <span className="text-[var(--text-muted)]">
                  ({(file.size / 1024).toFixed(1)} KB)
                </span>
              </p>
            ) : (
              <>
                <span className="text-3xl mb-2">📁</span>
                <p className="text-sm text-[var(--text-secondary)]">
                  Drag &amp; drop your schema file or{" "}
                  <span className="text-[var(--accent-blue)] font-medium">
                    browse
                  </span>
                </p>
              </>
            )}
          </div>

          <div className="flex justify-end gap-3 mt-2">
            <button type="button" onClick={onClose} className="btn-secondary">
              Cancel
            </button>
            <button
              type="submit"
              disabled={uploading || !file}
              className="btn-primary"
            >
              {uploading ? "Uploading…" : "Upload"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
