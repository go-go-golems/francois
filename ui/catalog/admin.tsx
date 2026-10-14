import React, { useEffect, useRef, useState } from "react";
import { useDispatch } from "react-redux";
import { api, Check, errorMessage, Release, SettingsDoc } from "./api";
import { navigate, show } from "./store";
import { bytes, date, datetime, Empty, Line, Link, reasonLabel, Row, short, useAct, useRoute } from "./ui";

const sections: [string, string][] = [
  ["", "Overview"],
  ["users", "Users"],
  ["instances", "Instances"],
  ["billing", "Billing"],
  ["usage", "Usage"],
  ["settings", "Settings"],
  ["audit", "Audit"],
];

export function Admin() {
  const { data: session } = api.useSessionQuery();
  const { path } = useRoute();
  const parts = path.split("/").filter(Boolean); // ["admin", section, id]
  const section = parts[1] ?? "";
  const id = parts[2];
  if (!session) return <main><Empty>Loading…</Empty></main>;
  if (session.user?.role !== "admin")
    return (
      <main className="lost">
        <h1>Administrators only.</h1>
        <Link className="u" to="/">
          Back to the catalog
        </Link>
      </main>
    );
  return (
    <main className="admin">
      <nav className="views admin-nav" aria-label="Administration">
        {sections.map(([s, label]) => (
          <Link key={s} to={"/admin" + (s ? "/" + s : "")} aria-current={section === s ? "page" : undefined}>
            {label}
          </Link>
        ))}
      </nav>
      {section === "" && <Overview />}
      {section === "users" && (id ? <UserDetail id={id} /> : <Users />)}
      {section === "instances" && (id ? <InstanceDetail id={id} /> : <AdminInstances />)}
      {section === "billing" && <Billing />}
      {section === "usage" && <Usage />}
      {section === "settings" && <Settings />}
      {section === "audit" && <Audit />}
    </main>
  );
}

function Tiles({ items }: { items: [React.ReactNode, string][] }) {
  return (
    <div className="stat-tiles">
      {items.map(([n, label]) => (
        <div className="stat-tile" key={label}>
          <span className="stat-n">{n}</span>
          <span className="stat-label">{label}</span>
        </div>
      ))}
    </div>
  );
}

function Overview() {
  const { data: o } = api.useOverviewQuery(undefined, { pollingInterval: 15000 });
  if (!o) return <Empty>Loading…</Empty>;
  return (
    <section className="stats">
      <div className="stats-head">
        <h1>Operations</h1>
        <span className="muted">period {o.usage.period}</span>
      </div>
      <Tiles items={[[o.users, "accounts"], [o.instances, "instances"], [o.activeViews, "live views"], [o.published, "published releases"], [o.drafts, "drafts"], [o.disabled, "disabled accounts"]]} />
      <Tiles
        items={[
          [o.usage.calls, "model calls"],
          [o.usage.credits, "credits charged"],
          [o.usage.inputTokens + o.usage.outputTokens, "tokens"],
          [o.usage.failed, "failed calls"],
          [o.usage.review, "calls in review"],
          [o.modelsAvailable, "provider models"],
        ]}
      />
      <Tiles
        items={[
          [o.billingEnabled ? "on" : "off", "billing"],
          [o.billing.pending, "pending events"],
          [o.billing.errors, "event errors"],
          [o.billing.openCheckouts, "open checkouts"],
          [o.billing.activeSubscriptions, "active subscriptions"],
          [o.billing.oldestPending ? datetime(o.billing.oldestPending) : "—", "oldest pending"],
        ]}
      />
      <p className="stats-note muted">
        Publish from the <Link className="u" to="/studio">Studio</Link>, approve models and allowances in{" "}
        <Link className="u" to="/admin/settings">Settings</Link>.
      </p>
    </section>
  );
}

function Users() {
  const [q, setQ] = useState("");
  const { data = [], isFetching } = api.useAdminUsersQuery(q);
  return (
    <section className="library">
      <div className="lib-head">
        <h2>Users</h2>
        <div className="lib-search">
          <input placeholder="search email, name, id …" value={q} onChange={(e) => setQ(e.target.value)} aria-label="Search users" />
        </div>
      </div>
      <div className="lib-list">
        {data.map(({ user: u, access }) => (
          <div className={"lib-row" + (u.disabled ? " is-unlisted" : "")} key={u.id}>
            <div className="lib-main">
              <Link to={"/admin/users/" + u.id}>
                <strong>{u.name || u.email || u.id}</strong>
              </Link>
              <span className="muted small">
                {u.email} {u.emailVerified ? "" : "· unverified"}
              </span>
            </div>
            <span className="lib-tags">{u.role}</span>
            <span className={"lib-status" + (access.allowed ? " on" : u.disabled ? " bad" : "")}>{reasonLabel(access.reason)}</span>
            <span className="lib-added">{date(u.lastLoginAt ?? u.createdAt)}</span>
            <div className="lib-actions">
              <Link className="btn btn-soft" to={"/admin/users/" + u.id}>
                Manage
              </Link>
            </div>
          </div>
        ))}
      </div>
      {!isFetching && data.length === 0 && <Empty>No accounts match.</Empty>}
    </section>
  );
}

function UserDetail({ id }: { id: string }) {
  const { data: d, isLoading } = api.useAdminUserQuery(id);
  const { data: session } = api.useSessionQuery();
  const [patch] = api.usePatchUserMutation();
  const [revoke] = api.useRevokeSessionsMutation();
  const [grant] = api.useGrantAccessMutation();
  const [revokeGrants] = api.useRevokeGrantsMutation();
  const [reconcile] = api.useReconcileMutation();
  const [adjust] = api.useAdjustMutation();
  const [days, setDays] = useState("30");
  const [reason, setReason] = useState("");
  const [credits, setCredits] = useState("50");
  const [creditReason, setCreditReason] = useState("");
  const act = useAct();
  if (isLoading || !d) return <Empty>Loading…</Empty>;
  const u = d.user;
  const self = session?.user?.id === u.id;
  return (
    <section className="sheet">
      <div className="stats-head">
        <h1>{u.name || u.email || u.id}</h1>
        <Link className="u" to="/admin/users">
          All users
        </Link>
      </div>
      <Row label="Identity" sub="owned by the identity provider">
        <div className="v-list">
          <Line k="Email">
            {u.email || "—"} {u.emailVerified ? <span className="muted">verified</span> : <span className="warn">unverified</span>}
          </Line>
          <Line k="User ID" mono>{u.id}</Line>
          <Line k="Issuer / subject" mono>
            {short(u.issuer, 40)} · {u.subject}
          </Line>
          <Line k="Created / last login">
            {date(u.createdAt)} · {datetime(u.lastLoginAt)}
          </Line>
          <Line k="Active sessions">{d.sessions}</Line>
        </div>
      </Row>
      <Row label="Access" sub={reasonLabel(d.access.reason)}>
        <div className="v-list">
          <Line k="Role">{u.role}</Line>
          <Line k="State">{u.disabled ? "disabled" : "enabled"}</Line>
          <Line k="Decision">
            {d.access.allowed ? "allowed" : "denied"} · {reasonLabel(d.access.reason)} {d.access.until ? "· until " + date(d.access.until) : ""}
          </Line>
          <Line k="Stripe customer" mono>{d.stripeCustomerId || "—"}</Line>
          {(d.subscriptions ?? []).map((s: any) => (
            <Line key={s.id} k={<span className="mono">{short(s.id, 18)}</span>}>
              {s.status} · until {date(s.accessUntil)} {s.cancelAtPeriodEnd ? "· cancels" : ""}
            </Line>
          ))}
          {(d.complimentary ?? []).map((g: any) => (
            <Line key={g.id} k="Complimentary">
              until {date(g.until)} · {g.reason} {g.revokedAt ? "· revoked" : ""}
            </Line>
          ))}
        </div>
        <div className="actions">
          <select
            className="btn btn-soft"
            aria-label="Role"
            disabled={self}
            value={u.role}
            onChange={(e) => act(patch({ id, role: e.target.value }).unwrap(), "Role updated")}
          >
            <option value="subscriber">Subscriber</option>
            <option value="creator">Creator (studio)</option>
            <option value="admin">Administrator</option>
          </select>
          <button className="btn btn-soft" disabled={self} onClick={() => confirm(u.disabled ? "Re-enable this account?" : "Disable hosted access, downloads and runtime grants?") && act(patch({ id, disabled: !u.disabled }).unwrap(), u.disabled ? "Account enabled" : "Account disabled")}>
            {u.disabled ? "Enable" : "Disable"}
          </button>
          <button className="btn btn-soft" onClick={() => act(revoke(id).unwrap().then((r) => r), "Sessions, grants and live views revoked")}>
            Revoke sessions
          </button>
          <button className="btn btn-soft" onClick={() => act(reconcile(id).unwrap(), "Reconciliation queued")}>
            Reconcile billing
          </button>
        </div>
        <form
          className="inline-form"
          onSubmit={(e) => {
            e.preventDefault();
            void act(grant({ id, days: Number(days), reason }).unwrap(), "Complimentary access granted").then(() => setReason(""));
          }}
        >
          <input aria-label="Days" className="num" type="number" min={1} max={3660} value={days} onChange={(e) => setDays(e.target.value)} />
          <span className="muted">days of complimentary access because</span>
          <input aria-label="Reason" required placeholder="reason" value={reason} onChange={(e) => setReason(e.target.value)} />
          <button className="btn btn-ink">Grant</button>
          {(d.complimentary ?? []).some((g: any) => !g.revokedAt) && (
            <button type="button" className="tlink quiet" onClick={() => act(revokeGrants(id).unwrap(), "Complimentary access revoked")}>
              revoke all
            </button>
          )}
        </form>
      </Row>
      <Row label="Model allowance" sub={`period ${d.allowance?.period}`}>
        <div className="v-list">
          <Line k="Remaining">{d.allowance?.remaining}</Line>
          <Line k="Used / reserved">
            {d.allowance?.used} / {d.allowance?.reserved}
          </Line>
          <Line k="Included + adjustments">
            {d.allowance?.included} + {d.allowance?.adjustments}
          </Line>
        </div>
        <form
          className="inline-form"
          onSubmit={(e) => {
            e.preventDefault();
            void act(adjust({ userId: id, credits: Number(credits), reason: creditReason }).unwrap(), "Allowance adjusted").then(() => setCreditReason(""));
          }}
        >
          <input aria-label="Credits" className="num" type="number" value={credits} onChange={(e) => setCredits(e.target.value)} />
          <span className="muted">credits because</span>
          <input aria-label="Adjustment reason" required placeholder="reason" value={creditReason} onChange={(e) => setCreditReason(e.target.value)} />
          <button className="btn btn-ink">Adjust</button>
        </form>
      </Row>
      <Row label="Instances" sub={`${d.instances.length} memberships`}>
        <div className="v-list">
          {d.instances.map((i: any) => (
            <Line key={i.id} k={<Link className="u" to={"/admin/instances/" + i.id}>{i.title}</Link>}>
              {i.role} · {i.artifactId} v{i.version} · {i.state}
            </Line>
          ))}
          {d.instances.length === 0 && <span className="muted">None</span>}
        </div>
      </Row>
      <Row label="Recent model calls">
        <Calls calls={d.calls ?? []} />
      </Row>
      <Row label="Audit">
        <AuditList entries={d.audit ?? []} />
      </Row>
    </section>
  );
}

function AdminInstances() {
  const [q, setQ] = useState("");
  const { data = [] } = api.useAdminInstancesQuery(q, { pollingInterval: 15000 });
  const [setState] = api.useSetInstanceStateMutation();
  const act = useAct();
  return (
    <section className="library">
      <div className="lib-head">
        <h2>Instances</h2>
        <div className="lib-search">
          <input placeholder="search title, owner, app …" value={q} onChange={(e) => setQ(e.target.value)} aria-label="Search instances" />
        </div>
      </div>
      <div className="lib-list">
        {data.map((i) => (
          <div className={"lib-row" + (i.state !== "active" ? " is-unlisted" : "")} key={i.id}>
            <div className="lib-main">
              <Link to={"/admin/instances/" + i.id}>
                <strong>{i.title}</strong>
              </Link>
              <span className="muted small">
                {i.artifactId} v{i.version} · {i.ownerName} · {i.kind}
              </span>
            </div>
            <span className="lib-tags">{i.members} members</span>
            <span className={"lib-status" + (i.activeViews ? " on" : "")}>{i.activeViews ? `${i.activeViews} live` : "idle"}</span>
            <span className={"lib-status" + (i.state === "active" ? "" : " bad")}>{i.state}</span>
            <div className="lib-actions">
              {i.state === "active" ? (
                <button className="btn btn-soft" onClick={() => confirm("Suspend this instance? Data is kept; live views close.") && act(setState({ id: i.id, action: "suspend" }).unwrap(), "Instance suspended")}>
                  Suspend
                </button>
              ) : (
                <button className="btn btn-soft" onClick={() => act(setState({ id: i.id, action: "resume" }).unwrap(), "Instance resumed")}>
                  Resume
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
      {data.length === 0 && <Empty>No instances.</Empty>}
    </section>
  );
}

function InstanceDetail({ id }: { id: string }) {
  const { data: i } = api.useAdminInstanceQuery(id);
  if (!i) return <Empty>Loading…</Empty>;
  return (
    <section className="sheet">
      <div className="stats-head">
        <h1>{i.title}</h1>
        <Link className="u" to="/admin/instances">
          All instances
        </Link>
      </div>
      <Row label="Instance" sub={i.state}>
        <div className="v-list">
          <Line k="ID" mono>{i.id}</Line>
          <Line k="Origin" mono>{i.origin}</Line>
          <Line k="Release">
            {i.artifactId} v{i.version} <span className="mono muted">{i.releaseId}</span>
          </Line>
          <Line k="Owner">{i.ownerName}</Line>
          <Line k="Live views">{i.activeViews}</Line>
        </div>
        <p className="muted small">Administrators inspect metadata here; customer pages are never executed on the portal origin.</p>
      </Row>
      <Row label="Storage">
        <div className="v-list">
          <Line k="Documents">
            {i.usage?.documents} · {bytes(i.usage?.documentBytes ?? 0)}
          </Line>
          <Line k="Assets">
            {i.usage?.assets} · {bytes(i.usage?.assetBytes ?? 0)}
          </Line>
          <Line k="Active grants">{i.usage?.activeGrants}</Line>
        </div>
      </Row>
      <Row label="Members">
        <div className="v-list">
          {(i.membersList ?? []).map((m) => (
            <Line key={m.userId} k={<Link className="u" to={"/admin/users/" + m.userId}>{m.name || m.email}</Link>}>
              {m.role}
            </Line>
          ))}
        </div>
      </Row>
      <Row label="Recent failures">
        <Calls calls={i.recentFailures ?? []} />
      </Row>
      <Row label="Audit">
        <AuditList entries={i.audit ?? []} />
      </Row>
    </section>
  );
}

function Billing() {
  const [state, setState] = useState("");
  const { data } = api.useBillingQuery(state, { pollingInterval: 10000 });
  const [retry] = api.useRetryEventMutation();
  const act = useAct();
  if (!data) return <Empty>Loading…</Empty>;
  const h = data.health;
  return (
    <section className="stats">
      <div className="stats-head">
        <h1>Billing</h1>
        <span className="muted mono">{data.enabled ? data.priceId : "not configured"}</span>
      </div>
      <Tiles items={[[h.customers, "customers"], [h.activeSubscriptions, "active subscriptions"], [h.openCheckouts, "open checkouts"], [h.pending, "pending events"], [h.errors, "event errors"], [h.oldestPending ? datetime(h.oldestPending) : "—", "oldest pending"]]} />
      <p className="muted small">
        Stripe is authoritative. Use the Stripe Dashboard or Customer Portal for refunds and plan changes; reconcile a user from their account page.
      </p>
      <div className="stats-block">
        <div className="views">
          {["", "pending", "processed", "error", "ignored"].map((s) => (
            <button key={s} className="tlink quiet" aria-pressed={state === s} onClick={() => setState(s)}>
              {s || "all events"}
            </button>
          ))}
        </div>
        <div className="srow shead">
          <span className="sname">Event</span>
          <span className="scol">Type</span>
          <span className="scol">State</span>
          <span className="snum">Tries</span>
          <span className="scol">Received</span>
          <span className="scol" />
        </div>
        {data.events.map((e: any) => (
          <div className="srow" key={e.eventId}>
            <span className="sname mono">{e.eventId}</span>
            <span className="scol">{e.type}</span>
            <span className={"scol" + (e.state === "error" ? " warn" : "")}>
              {e.state}
              {e.errorCode ? " · " + e.errorCode : ""}
            </span>
            <span className="snum">{e.attempts}</span>
            <span className="scol">{datetime(e.receivedAt)}</span>
            <span className="scol">
              {(e.state === "error" || e.state === "pending") && (
                <button className="tlink quiet" onClick={() => act(retry(e.eventId).unwrap(), "Retry queued")}>
                  retry
                </button>
              )}
            </span>
          </div>
        ))}
        {data.events.length === 0 && <Empty>No events.</Empty>}
      </div>
      <div className="stats-block">
        <h2>Checkout attempts</h2>
        {data.checkouts.map((c: any) => (
          <div className="srow" key={c.id}>
            <span className="sname mono">{c.id}</span>
            <span className="scol mono">{short(c.userId, 14)}</span>
            <span className="scol">{c.state}</span>
            <span className="scol">{datetime(c.createdAt)}</span>
          </div>
        ))}
        {data.checkouts.length === 0 && <Empty>No checkouts.</Empty>}
      </div>
    </section>
  );
}

function Calls({ calls }: { calls: any[] }) {
  if (!calls.length) return <span className="muted">None</span>;
  return (
    <div className="stats-block">
      <div className="srow shead">
        <span className="scol">When</span>
        <span className="sname">Model</span>
        <span className="scol">State</span>
        <span className="snum">Credits</span>
        <span className="snum">Tokens</span>
      </div>
      {calls.map((c) => (
        <div className="srow" key={c.id}>
          <span className="scol">{datetime(c.createdAt)}</span>
          <span className="sname mono">{c.modelUsed || c.model}</span>
          <span className={"scol" + (c.state === "failed" || c.state === "review" ? " warn" : "")}>
            {c.state}
            {c.errorCode ? " · " + c.errorCode : ""}
          </span>
          <span className="snum">{c.credits}</span>
          <span className="snum">{(c.inputTokens ?? 0) + (c.outputTokens ?? 0)}</span>
        </div>
      ))}
    </div>
  );
}

function Usage() {
  const [user, setUser] = useState("");
  const { data } = api.useUsageQuery(user, { pollingInterval: 10000 });
  if (!data) return <Empty>Loading…</Empty>;
  const s = data.summary;
  return (
    <section className="stats">
      <div className="stats-head">
        <h1>Model usage</h1>
        <span className="muted">period {s.period}</span>
      </div>
      <Tiles items={[[s.calls, "calls"], [s.credits, "credits charged"], [s.activeUsers, "active users"], [s.inputTokens, "input tokens"], [s.outputTokens, "output tokens"], [s.cost ? "$" + s.cost.toFixed(4) : "—", "provider cost"]]} />
      <p className="muted small">Provider cost may be zero or estimated for subscription/OAuth routes; included credits are the customer allowance.</p>
      <div className="lib-search">
        <input placeholder="filter by user id …" value={user} onChange={(e) => setUser(e.target.value)} aria-label="Filter by user" />
      </div>
      <div className="stats-block">
        <div className="srow shead">
          <span className="scol">When</span>
          <span className="scol">User</span>
          <span className="sname">Model</span>
          <span className="scol">State</span>
          <span className="snum">Credits</span>
          <span className="snum">In</span>
          <span className="snum">Out</span>
          <span className="snum">Rev</span>
        </div>
        {data.calls.map((c) => (
          <div className="srow" key={c.id}>
            <span className="scol">{datetime(c.createdAt)}</span>
            <span className="scol">
              <Link className="u" to={"/admin/users/" + c.userId}>
                {short(c.userEmail || c.userId, 18)}
              </Link>
            </span>
            <span className="sname mono">{c.modelUsed || c.model}</span>
            <span className={"scol" + (c.state === "failed" || c.state === "review" ? " warn" : "")}>
              {c.state}
              {c.errorCode ? " · " + c.errorCode : ""}
            </span>
            <span className="snum">{c.credits}</span>
            <span className="snum">{c.inputTokens}</span>
            <span className="snum">{c.outputTokens}</span>
            <span className="snum">{c.settingsRevision}</span>
          </div>
        ))}
        {data.calls.length === 0 && <Empty>No model calls yet.</Empty>}
      </div>
    </section>
  );
}

const thinkingLevels = ["off", "minimal", "low", "medium", "high", "xhigh", "max"];

function Settings() {
  const { data } = api.useSettingsQuery();
  const [save, { isLoading }] = api.useSaveSettingsMutation();
  const [doc, setDoc] = useState<SettingsDoc>();
  const [error, setError] = useState("");
  const [raw, setRaw] = useState(false);
  const [rawText, setRawText] = useState("");
  const dispatch = useDispatch();
  useEffect(() => {
    if (data) {
      setDoc(structuredClone(data.active.document));
      setRawText(JSON.stringify(data.active.document, null, 2));
    }
  }, [data?.active.revision]);
  if (!data || !doc) return <Empty>Loading…</Empty>;
  const upd = (f: (d: SettingsDoc) => void) => {
    const next = structuredClone(doc);
    f(next);
    setDoc(next);
    setRawText(JSON.stringify(next, null, 2));
  };
  const submit = async () => {
    setError("");
    try {
      const body = raw ? JSON.parse(rawText) : doc;
      const rev = await save(body).unwrap();
      dispatch(show({ text: `Settings revision ${rev.revision} is active.` }));
    } catch (e) {
      setError(e instanceof SyntaxError ? "Settings JSON is invalid: " + e.message : errorMessage(e));
    }
  };
  const approved = new Set(doc.models.allowed.map((m) => m.selector));
  const num = (v: string) => (v === "" ? 0 : Number(v));
  return (
    <section className="sheet">
      <div className="stats-head">
        <h1>Settings</h1>
        <span className="muted">
          revision {data.active.revision} · {data.active.revision ? datetime(data.active.activatedAt) : "defaults"}
        </span>
      </div>
      <Row label="Secrets" sub="deployment configuration">
        <div className="v-list">
          {Object.entries(data.secrets).map(([k, v]) => (
            <Line key={k} k={k}>
              {v ? "configured" : "not configured"}
            </Line>
          ))}
        </div>
        <p className="muted small">Credentials live in deployment secret files and are never shown or edited here.</p>
      </Row>
      <div className="seg settings-mode">
        <button aria-pressed={!raw} onClick={() => setRaw(false)}>
          Form
        </button>
        <button aria-pressed={raw} onClick={() => setRaw(true)}>
          JSON
        </button>
      </div>
      {raw ? (
        <Row label="Document" sub="validated strictly on save">
          <textarea className="code-area" rows={28} spellCheck={false} value={rawText} onChange={(e) => setRawText(e.target.value)} />
        </Row>
      ) : (
        <>
          <Row label="Approved models" sub="from the operator's Pi configuration">
            {data.providerModels.length === 0 && <p className="warn">The model worker reports no models.</p>}
            <div className="model-table">
              {data.providerModels.map((pm) => {
                const m = doc.models.allowed.find((a) => a.selector === pm.selector);
                return (
                  <div className={"model-line" + (m ? " on" : "")} key={pm.selector}>
                    <label className="check">
                      <input
                        type="checkbox"
                        checked={!!m}
                        onChange={(e) =>
                          upd((d) => {
                            if (e.target.checked) d.models.allowed.push({ selector: pm.selector, label: pm.name, thinking: pm.thinkingLevels?.filter((t) => t !== "off") ?? [], credits: 1 });
                            else {
                              d.models.allowed = d.models.allowed.filter((a) => a.selector !== pm.selector);
                              for (const t of Object.keys(d.models.tiers)) if (d.models.tiers[t] === pm.selector) delete d.models.tiers[t];
                            }
                          })
                        }
                      />
                      <span className="mono">{pm.selector}</span>
                    </label>
                    {m && (
                      <span className="model-opts">
                        <input aria-label="Label" value={m.label} onChange={(e) => upd((d) => (d.models.allowed.find((a) => a.selector === pm.selector)!.label = e.target.value))} />
                        <input aria-label="Credits" className="num" type="number" min={1} value={m.credits} onChange={(e) => upd((d) => (d.models.allowed.find((a) => a.selector === pm.selector)!.credits = num(e.target.value)))} />
                        <span className="muted">credits</span>
                        <span className="seg small">
                          {thinkingLevels
                            .filter((t) => (pm.thinkingLevels ?? []).includes(t) || m.thinking.includes(t))
                            .map((t) => (
                              <button
                                key={t}
                                aria-pressed={m.thinking.includes(t)}
                                onClick={() =>
                                  upd((d) => {
                                    const a = d.models.allowed.find((x) => x.selector === pm.selector)!;
                                    a.thinking = a.thinking.includes(t) ? a.thinking.filter((x) => x !== t) : [...a.thinking, t];
                                  })
                                }
                              >
                                {t}
                              </button>
                            ))}
                        </span>
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
            <div className="tiers">
              {["quick", "default", "complex"].map((tier) => (
                <label key={tier} className="field">
                  <span>{tier} tier</span>
                  <select value={doc.models.tiers[tier] ?? ""} onChange={(e) => upd((d) => (e.target.value ? (d.models.tiers[tier] = e.target.value) : delete d.models.tiers[tier]))}>
                    <option value="">— none —</option>
                    {[...approved].map((s) => (
                      <option key={s} value={s}>
                        {s}
                      </option>
                    ))}
                  </select>
                </label>
              ))}
            </div>
          </Row>
          <Row label="Allowance" sub="per user, calendar month (UTC)">
            <div className="grid-fields">
              <NumField label="Included credits / month" value={doc.allowance.monthlyCredits} set={(v) => upd((d) => (d.allowance.monthlyCredits = v))} />
              <NumField label="Daily safeguard (0 = off)" value={doc.allowance.dailyCredits} set={(v) => upd((d) => (d.allowance.dailyCredits = v))} />
              <NumField label="Concurrent calls per user" value={doc.allowance.perUserConcurrency} set={(v) => upd((d) => (d.allowance.perUserConcurrency = v))} />
              <NumField label="Concurrent calls globally" value={doc.allowance.globalConcurrency} set={(v) => upd((d) => (d.allowance.globalConcurrency = v))} />
              <NumField label="Max tool rounds" value={doc.allowance.maxToolRounds} set={(v) => upd((d) => (d.allowance.maxToolRounds = v))} />
            </div>
          </Row>
          <Row label="Limits" sub="hosted resources">
            <div className="grid-fields">
              <NumField label="Instances per user" value={doc.limits.instancesPerUser} set={(v) => upd((d) => (d.limits.instancesPerUser = v))} />
              <NumField label="Members per instance" value={doc.limits.membersPerInstance} set={(v) => upd((d) => (d.limits.membersPerInstance = v))} />
              <NumField label="Documents per instance" value={doc.limits.documentsPerInstance} set={(v) => upd((d) => (d.limits.documentsPerInstance = v))} />
              <NumField label="Asset MiB per instance" value={Math.round(doc.limits.assetBytesPerInstance / (1 << 20))} set={(v) => upd((d) => (d.limits.assetBytesPerInstance = v * (1 << 20)))} />
              <NumField label="Max HTML KiB" value={Math.round(doc.limits.maxHtmlBytes / 1024)} set={(v) => upd((d) => (d.limits.maxHtmlBytes = v * 1024))} />
            </div>
          </Row>
          <Row label="Publishing">
            <label className="check">
              <input type="checkbox" checked={doc.publishing.allowPinnedAfterWithdraw} onChange={(e) => upd((d) => (d.publishing.allowPinnedAfterWithdraw = e.target.checked))} /> Instances pinned to
              a withdrawn release keep running
            </label>
          </Row>
        </>
      )}
      {error && <p className="notice bad">{error}</p>}
      <div className="form-foot sticky">
        <span className="muted small">Saving validates and activates a new revision atomically; calls record the revision they were admitted under.</span>
        <button className="btn btn-ink" onClick={submit} disabled={isLoading}>
          {isLoading ? "Saving…" : "Save & activate"}
        </button>
      </div>
      <Row label="History">
        <div className="v-list">
          {data.history.map((h) => (
            <Line key={h.revision} k={"revision " + h.revision}>
              {datetime(h.activatedAt)} · {h.document.models.allowed.length} models · {h.document.allowance.monthlyCredits} credits/month
            </Line>
          ))}
          {data.history.length === 0 && <span className="muted">Defaults only; no revision saved yet.</span>}
        </div>
      </Row>
    </section>
  );
}

function NumField({ label, value, set }: { label: string; value: number; set: (v: number) => void }) {
  return (
    <label className="field">
      <span>{label}</span>
      <input type="number" value={value} onChange={(e) => set(e.target.value === "" ? 0 : Number(e.target.value))} />
    </label>
  );
}

function AuditList({ entries }: { entries: any[] }) {
  if (!entries.length) return <span className="muted">No audit entries.</span>;
  return (
    <div className="stats-block">
      {entries.map((e) => (
        <div className="srow" key={e.id}>
          <span className="scol">{datetime(e.createdAt)}</span>
          <span className="scol">{e.action}</span>
          <span className="sname mono">{e.subject}</span>
          <span className="scol mono muted">{e.actorId ? short(e.actorId, 14) : "operator"}</span>
          <span className="sname mono muted">{JSON.stringify(e.summary) === "{}" ? "" : JSON.stringify(e.summary)}</span>
        </div>
      ))}
    </div>
  );
}

function Audit() {
  const [subject, setSubject] = useState("");
  const { data = [] } = api.useAuditQuery(subject);
  return (
    <section className="library">
      <div className="lib-head">
        <h2>Audit</h2>
        <div className="lib-search">
          <input placeholder="subject id …" value={subject} onChange={(e) => setSubject(e.target.value)} aria-label="Filter by subject" />
        </div>
      </div>
      <AuditList entries={data} />
    </section>
  );
}
