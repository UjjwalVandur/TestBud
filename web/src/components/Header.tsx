"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { checkHealth } from "@/lib/api";

export default function Header() {
  const pathname = usePathname();
  const [showSettings, setShowSettings] = useState(false);
  const [apiKey, setApiKey] = useState("");
  const [healthOk, setHealthOk] = useState<boolean | null>(null);

  useEffect(() => {
    setApiKey(localStorage.getItem("testbud_api_key") ?? "");
    checkHealth()
      .then(() => setHealthOk(true))
      .catch(() => setHealthOk(false));
  }, []);

  function saveKey() {
    localStorage.setItem("testbud_api_key", apiKey);
    setShowSettings(false);
    window.location.reload();
  }

  const links = [
    { href: "/", label: "Dashboard" },
    { href: "/schemas", label: "Schemas" },
  ];

  return (
    <>
      <header className="sticky top-0 z-50 border-b border-[var(--border-subtle)] bg-[var(--bg-primary)]/80 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-6">
          {/* Logo */}
          <Link href="/" className="flex items-center gap-2.5 no-underline">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-[var(--accent-blue)] to-[var(--accent-violet)]">
              <span className="text-sm font-bold text-white">TB</span>
            </div>
            <span className="text-lg font-bold gradient-text">TestBud</span>
          </Link>

          {/* Nav links */}
          <nav className="flex items-center gap-1">
            {links.map((link) => {
              const active = pathname === link.href;
              return (
                <Link
                  key={link.href}
                  href={link.href}
                  className={`rounded-lg px-4 py-2 text-sm font-medium no-underline transition-colors ${
                    active
                      ? "bg-[var(--accent-blue)]/10 text-[var(--accent-blue)]"
                      : "text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-white/5"
                  }`}
                >
                  {link.label}
                </Link>
              );
            })}
          </nav>

          {/* Right side */}
          <div className="flex items-center gap-4">
            {/* Health indicator */}
            <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
              <span
                className={`inline-block h-2 w-2 rounded-full ${
                  healthOk === true
                    ? "bg-[var(--accent-emerald)] animate-pulse-glow"
                    : healthOk === false
                    ? "bg-[var(--accent-rose)]"
                    : "bg-[var(--text-muted)]"
                }`}
              />
              {healthOk === true ? "API Online" : healthOk === false ? "API Offline" : "Checking…"}
            </div>

            {/* Settings button */}
            <button
              onClick={() => setShowSettings(true)}
              className="btn-secondary !py-2 !px-3 text-xs"
            >
              ⚙ Settings
            </button>
          </div>
        </div>
      </header>

      {/* Settings modal */}
      {showSettings && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm animate-slide-up">
          <div className="glass-card w-full max-w-md p-8">
            <h2 className="mb-1 text-xl font-bold">API Settings</h2>
            <p className="mb-6 text-sm text-[var(--text-secondary)]">
              Configure your API key to authenticate with the TestBud backend.
            </p>

            <label className="mb-2 block text-xs font-medium text-[var(--text-secondary)]">
              API Key
            </label>
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="Enter your API key…"
              className="mb-6 w-full"
            />

            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setShowSettings(false)}
                className="btn-secondary"
              >
                Cancel
              </button>
              <button onClick={saveKey} className="btn-primary">
                Save
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
