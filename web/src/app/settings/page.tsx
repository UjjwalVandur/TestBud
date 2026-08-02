"use client";

import { useState, useEffect } from "react";

import { useAuth } from "@clerk/nextjs";

export default function SettingsPage() {
  const { getToken } = useAuth();
  const [apiKey, setApiKey] = useState<string>("Loading...");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    async function fetchKey() {
      try {
        const token = await getToken();
        if (token) {
          // In a real app we'd fetch this from the Go backend.
          // Since our middleware lazily creates the user and we didn't add a GET /api/me route,
          // the user will need to use their token directly for now if hitting the CLI,
          // or we can just tell them how to get it.
          setApiKey(token);
        }
      } catch {
        setApiKey("Error loading API Key");
      }
    }
    fetchKey();
  }, [getToken]);

  const copyToClipboard = () => {
    navigator.clipboard.writeText(apiKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="mx-auto max-w-3xl space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Settings</h1>
        <p className="mt-2 text-[var(--text-secondary)]">
          Manage your account and authentication credentials.
        </p>
      </div>

      <div className="glass-card p-6">
        <h2 className="mb-4 text-xl font-bold">API Key</h2>
        <p className="mb-4 text-sm text-[var(--text-secondary)]">
          Use this API key (Bearer token) to authenticate with the TestBud CLI or API directly.
        </p>

        <div className="flex items-center gap-3">
          <input
            type="text"
            readOnly
            value={apiKey}
            className="flex-1 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-primary)] px-4 py-3 font-mono text-sm focus:outline-none"
          />
          <button onClick={copyToClipboard} className="btn-primary whitespace-nowrap">
            {copied ? "Copied!" : "Copy Key"}
          </button>
        </div>
      </div>
    </div>
  );
}
