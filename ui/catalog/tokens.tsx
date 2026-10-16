import React, { useState } from "react";
import { api, SessionUser, TokenScope } from "./api";
import { date, Empty, useAct } from "./ui";

// What each scope lets a token do. The server enforces the same list per role
// (store.ScopesForRole) and never lets a scope exceed the owner's role.
const scopeInfo: Record<TokenScope, string> = {
  account: "your account, billing links and instances",
  studio: "artifacts, releases, media, journal posts and creators you may edit",
  "admin:read": "read customers, instances, billing, usage, settings and the audit log",
  "admin:write": "change them: roles, grants, instances, settings, credits",
};

export function scopesFor(role: SessionUser["role"]): TokenScope[] {
  switch (role) {
    case "admin":
      return ["studio", "account", "admin:read", "admin:write"];
    case "creator":
      return ["studio", "account"];
    case "subscriber":
      return ["account"];
    default:
      return [];
  }
}

const isAdminScope = (s: TokenScope) => s === "admin:read" || s === "admin:write";

// ApiTokens manages personal API tokens for scripts and agents. The secret is
// shown once; only its hash is stored.
export function ApiTokens({ role }: { role: SessionUser["role"] }) {
  const allowed = scopesFor(role);
  const { data = [], isLoading } = api.useTokensQuery();
  const [create] = api.useCreateTokenMutation();
  const [revoke] = api.useRevokeTokenMutation();
  const act = useAct();
  const [name, setName] = useState("");
  const [days, setDays] = useState("90");
  const [scopes, setScopes] = useState<TokenScope[]>(allowed.length === 1 ? allowed : []);
  const [secret, setSecret] = useState<{ token: string; scopes: TokenScope[] } | null>(null);
  if (allowed.length === 0) return <p className="muted">This account cannot create API tokens.</p>;
  const admin = scopes.some(isAdminScope);
  const toggle = (s: TokenScope) => setScopes(scopes.includes(s) ? scopes.filter((x) => x !== s) : [...scopes, s]);
  return (
    <div id="api-tokens" className="token-panel">
      <form
        className="token-form"
        onSubmit={async (e) => {
          e.preventDefault();
          const r = await act(create({ name, days: Number(days), scopes }).unwrap(), "Token created");
          if (r) {
            setSecret({ token: r.token, scopes: r.info.scopes });
            setName("");
          }
        }}
      >
        <div className="inline-form">
          <input placeholder="name, e.g. laptop agent" required value={name} onChange={(e) => setName(e.target.value)} />
          <input aria-label="Days" className="num" type="number" min={admin ? 1 : 0} max={admin ? 90 : 366} value={days} onChange={(e) => setDays(e.target.value)} />
          <span className="muted">{admin ? "days (1–90 for admin scopes)" : "days (0 = no expiry)"}</span>
        </div>
        <fieldset className="scope-list">
          <legend className="muted small">Scopes</legend>
          {allowed.map((s) => (
            <label key={s} className="scope-option">
              <input type="checkbox" checked={scopes.includes(s)} onChange={() => toggle(s)} />
              <span className="mono">{s}</span> <span className="muted small">— {scopeInfo[s]}</span>
            </label>
          ))}
        </fieldset>
        <button className="btn btn-ink" disabled={scopes.length === 0}>
          Create token
        </button>
      </form>
      {secret && (
        <div className="notice token-once">
          <p>Copy this token now; it is not shown again. Store it somewhere safe, such as a password manager or Vault.</p>
          <pre className="code-inline">{secret.token}</pre>
          <pre className="code-inline">{`export FRANCOIS_TOKEN=…   # the token above
curl -H "Authorization: Bearer $FRANCOIS_TOKEN" ${location.origin}/api/me`}</pre>
          {secret.scopes.includes("studio") && (
            <pre className="code-inline">{`francois catalog push --server ${location.origin} \\\n  --artifact-id <id> --version 1.0.0 --html dist/app.html --publish`}</pre>
          )}
          <button type="button" className="tlink" onClick={() => setSecret(null)}>
            Done
          </button>
        </div>
      )}
      {isLoading ? (
        <Empty>Loading…</Empty>
      ) : data.length === 0 ? (
        <Empty>No tokens. A token lets a script or an agent act as you, limited to the scopes you choose.</Empty>
      ) : (
        <div className="lib-list">
          {data.map((t) => (
            <div className={"lib-row" + (t.revokedAt ? " is-unlisted" : "")} key={t.id}>
              <span className="lib-main">
                <strong>{t.name}</strong>
                <span className="muted small">
                  <span className="mono">{t.scopes.join(" ")}</span> · created {date(t.createdAt)} · {t.lastUsedAt ? "last used " + date(t.lastUsedAt) : "never used"}
                  {t.expiresAt ? " · expires " + date(t.expiresAt) : " · no expiry"}
                </span>
              </span>
              <span className={"lib-status" + (t.revokedAt ? " bad" : " on")}>{t.revokedAt ? "revoked" : "active"}</span>
              <div className="lib-actions">
                {!t.revokedAt && (
                  <button className="btn btn-soft" onClick={() => confirm(`Revoke ${t.name}?`) && act(revoke(t.id).unwrap(), "Token revoked")}>
                    Revoke
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
