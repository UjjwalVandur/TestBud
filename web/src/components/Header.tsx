"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { checkHealth } from "@/lib/api";
import { SignInButton, UserButton, useAuth } from "@clerk/nextjs";

export default function Header() {
  const pathname = usePathname();
  const { isSignedIn } = useAuth();
  const [healthOk, setHealthOk] = useState<boolean | null>(null);

  useEffect(() => {
    checkHealth()
      .then(() => setHealthOk(true))
      .catch(() => setHealthOk(false));
  }, []);

  const links = [
    { href: "/", label: "Dashboard" },
    { href: "/schemas", label: "Schemas" },
    { href: "/settings", label: "Settings" },
  ];

  return (
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

          {!isSignedIn && (
            <SignInButton mode="modal">
              <button className="btn-primary !py-2 !px-4 text-sm">
                Sign In
              </button>
            </SignInButton>
          )}
          {isSignedIn && (
            <UserButton />
          )}
        </div>
      </div>
    </header>
  );
}
