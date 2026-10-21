import React, { useEffect, useState } from "react";
import { api, errorCode, errorMessage, TokenScope } from "./api";
import { scopeInfo } from "./tokens";
import { Empty, loginHref, useAct } from "./ui";

const isAdminScope = (s: TokenScope) =>
  s === "admin:read" || s === "admin:write";

// minutesUntil is a short relative time for the expiry line.
function minutesUntil(iso: string): string {
  const m = Math.max(0, Math.round((Date.parse(iso) - Date.now()) / 60000));
  return m === 1 ? "1 minute" : `${m} minutes`;
}

// Device approves a CLI login: `francois catalog auth login` prints a link to
// /device?code=XXXX-XXXX. Approving mints a token for the waiting terminal.
export function Device() {
  const { data: session } = api.useSessionQuery();
  const code = new URLSearchParams(location.search).get("code") ?? "";
  useEffect(() => {
    if (!session?.brand) return;
    const previous = document.title;
    document.title = `Authorize a terminal · ${session.brand}`;
    return () => {
      document.title = previous;
    };
  }, [session?.brand]);
  if (!session)
    return (
      <main>
        <Empty>Loading…</Empty>
      </main>
    );
  if (!session.authenticated)
    return (
      <main className="lost">
        <h1>Sign in to authorize a terminal.</h1>
        <p>
          A command-line login is waiting for approval. Sign in, then check
          that the code matches your terminal.
        </p>
        <a className="btn btn-ink" href={loginHref()}>
          Sign in
        </a>
      </main>
    );
  return code ? <Approve code={code} /> : <EnterCode />;
}

function EnterCode() {
  const [value, setValue] = useState("");
  return (
    <main className="lost">
      <h1>Authorize a terminal.</h1>
      <form
        className="inline-form"
        onSubmit={(e) => {
          e.preventDefault();
          location.search = "?code=" + encodeURIComponent(value.trim());
        }}
      >
        <input
          aria-label="Code"
          placeholder="Code shown in your terminal, e.g. WDJB-MJHT"
          required
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
        <button className="btn btn-ink">Continue</button>
      </form>
    </main>
  );
}

function Approve({ code }: { code: string }) {
  const { data: session } = api.useSessionQuery();
  const { data: req, error, isLoading } = api.useDeviceRequestQuery(code);
  const [approve, { isLoading: approving }] = api.useApproveDeviceMutation();
  const [deny, { isLoading: denying }] = api.useDenyDeviceMutation();
  const act = useAct();
  const [scopes, setScopes] = useState<TokenScope[]>([]);
  const [days, setDays] = useState("90");
  const [done, setDone] = useState<"approved" | "denied" | null>(null);
  useEffect(() => {
    if (!req) return;
    setScopes(req.suggestedScopes);
    setDays(String(req.suggestedDays));
  }, [req]);
  if (done)
    return (
      <main className="lost">
        <h1>{done === "approved" ? "Done." : "Denied."}</h1>
        <p>
          {done === "approved"
            ? "You can return to your terminal; it receives the token in a few seconds."
            : "The terminal will stop waiting. No token was created."}
        </p>
      </main>
    );
  if (isLoading)
    return (
      <main>
        <Empty>Loading…</Empty>
      </main>
    );
  if (error || !req)
    return (
      <main className="lost">
        <h1>
          {errorCode(error) === "not_found"
            ? "This code is invalid or has expired."
            : "This login cannot be shown."}
        </h1>
        <p>
          {errorCode(error) === "not_found"
            ? "Run francois catalog auth login again for a new code."
            : errorMessage(error)}
        </p>
      </main>
    );
  if (req.allowedScopes.length === 0)
    return (
      <main className="lost">
        <h1>This account cannot create API tokens.</h1>
      </main>
    );
  const admin = scopes.some(isAdminScope);
  const toggle = (s: TokenScope) =>
    setScopes(
      scopes.includes(s) ? scopes.filter((x) => x !== s) : [...scopes, s],
    );
  const host = location.host;
  return (
    <main>
      <section className="intro">
        <h1>Authorize a terminal.</h1>
      </section>
      <form
        className="device-panel"
        onSubmit={async (e) => {
          e.preventDefault();
          const r = await act(
            approve({ code, scopes, days: Number(days) }).unwrap(),
          );
          if (r) setDone("approved");
        }}
      >
        <p>
          <strong>{req.clientName}</strong> wants to act as{" "}
          <strong>
            {session?.user?.name ?? "you"}
            {session?.user?.email ? ` (${session.user.email})` : ""}
          </strong>{" "}
          on <strong>{host}</strong>.
        </p>
        <div>
          <div className="muted small">
            Check that this matches the code in your terminal:
          </div>
          <div className="device-code">{req.userCode}</div>
        </div>
        <p className="muted small">
          Requested from {req.requestedFrom}, expires in{" "}
          {minutesUntil(req.expiresAt)}.
        </p>
        <fieldset className="scope-list">
          <legend className="muted small">Scopes</legend>
          {req.allowedScopes.map((s) => (
            <label key={s} className="scope-option">
              <input
                type="checkbox"
                checked={scopes.includes(s)}
                onChange={() => toggle(s)}
              />
              <span className="mono">{s}</span>{" "}
              <span className="muted small">— {scopeInfo[s]}</span>
            </label>
          ))}
        </fieldset>
        <div className="inline-form">
          <input
            aria-label="Days"
            className="num"
            type="number"
            min={admin ? 1 : 0}
            max={admin ? req.maxAdminDays : 366}
            value={days}
            onChange={(e) => setDays(e.target.value)}
          />
          <span className="muted">
            {admin
              ? `days (1–${req.maxAdminDays} for admin scopes)`
              : "days (0 = no expiry)"}
          </span>
        </div>
        <p className="notice">
          Only approve if you started this login yourself, just now. Anyone
          holding the resulting token can act as you within these scopes. You
          can revoke it on your account page at any time.
        </p>
        <div className="device-actions">
          <button
            className="btn btn-ink"
            disabled={scopes.length === 0 || approving || denying}
          >
            Approve
          </button>
          <button
            type="button"
            className="btn btn-soft"
            disabled={approving || denying}
            onClick={async () => {
              if (await act(deny(code).unwrap())) setDone("denied");
            }}
          >
            Deny
          </button>
        </div>
      </form>
    </main>
  );
}
