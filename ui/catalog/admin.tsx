import { AnalyticsPage } from "./analytics";
import React, { useEffect, useRef, useState } from "react";
import { useDispatch } from "react-redux";
import { api, Check, errorMessage, Plan, Release, SettingsDoc } from "./api";
import { navigate, show } from "./store";
import {
  bytes,
  date,
  datetime,
  Empty,
  Line,
  Link,
  reasonLabel,
  Row,
  short,
  useAct,
  useRoute,
} from "./ui";

const sections: [string, string][] = [
  ["", "Overview"],
  ["users", "Users"],
  ["instances", "Instances"],
  ["billing", "Billing"],
  ["usage", "Usage"],
  ["analytics", "Analytics"],
  ["settings", "Settings"],
  ["audit", "Audit"],
];

export function Admin() {
  const { data: session } = api.useSessionQuery();
  const { path } = useRoute();
  const parts = path.split("/").filter(Boolean); // ["admin", section, id]
  const section = parts[1] ?? "";
  const id = parts[2];
  if (!session)
    return (
      <main>
        <Empty>Loading…</Empty>
      </main>
    );
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
          <Link
            key={s}
            to={"/admin" + (s ? "/" + s : "")}
            aria-current={section === s ? "page" : undefined}
          >
            {label}
          </Link>
        ))}
      </nav>
      {section === "" && <Overview />}
      {section === "users" && (id ? <UserDetail id={id} /> : <Users />)}
      {section === "instances" &&
        (id ? <InstanceDetail id={id} /> : <AdminInstances />)}
      {section === "billing" && <Billing />}
      {section === "usage" && <Usage />}
      {section === "analytics" && <AnalyticsPage />}
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
  const { data: o } = api.useOverviewQuery(undefined, {
    pollingInterval: 15000,
  });
  if (!o) return <Empty>Loading…</Empty>;
  return (
    <section className="stats">
      <div className="stats-head">
        <h1>Operations</h1>
        <span className="muted">period {o.usage.period}</span>
      </div>
      <Tiles
        items={[
          [o.users, "accounts"],
          [o.instances, "instances"],
          [o.activeViews, "live views"],
          [o.published, "published releases"],
          [o.drafts, "drafts"],
          [o.disabled, "disabled accounts"],
        ]}
      />
      <Tiles
        items={[
          [o.usage.calls, "model calls"],

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
          [
            o.billing.oldestPending ? datetime(o.billing.oldestPending) : "—",
            "oldest pending",
          ],
        ]}
      />
      <p className="stats-note muted">
        Publish from the{" "}
        <Link className="u" to="/studio">
          Studio
        </Link>
        , approve models and allowances in{" "}
        <Link className="u" to="/admin/settings">
          Settings
        </Link>
        .
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
          <input
            placeholder="search email, name, id …"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="Search users"
          />
        </div>
      </div>
      <div className="lib-list">
        {data.map(({ user: u, customerClass, enabled }) => (
          <div
            className={"lib-row" + (u.disabled ? " is-unlisted" : "")}
            key={u.id}
          >
            <div className="lib-main">
              <Link to={"/admin/users/" + u.id}>
                <strong>{u.name || u.email || u.id}</strong>
              </Link>
              <span className="muted small">
                {u.email} {u.emailVerified ? "" : "· unverified"}
              </span>
            </div>
            <span className="lib-tags">{customerClass}</span>
            <span className={"lib-status" + (enabled ? " on" : " bad")}>
              {enabled ? "Enabled" : "Disabled"}
            </span>
            <span className="lib-added">
              {date(u.lastLoginAt ?? u.createdAt)}
            </span>
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
  const { data: d, isLoading, error } = api.useAdminUserQuery(id);
  const { data: session } = api.useSessionQuery();
  const [patch] = api.usePatchUserMutation();
  const [grant] = api.useGrantAccessMutation();
  const [endGrant] = api.useRevokeGrantsMutation();
  const [ceiling] = api.useTokenCeilingMutation();
  const [revoke] = api.useRevokeSessionsMutation();
  const [reconcile] = api.useReconcileMutation();
  const [limit, setLimit] = useState("");
  const [reason, setReason] = useState("");
  const [ceilingReason, setCeilingReason] = useState("");
  const [days, setDays] = useState("30");
  const act = useAct();
  if (error) return <Empty>{errorMessage(error)}</Empty>;
  if (isLoading || !d) return <Empty>Loading…</Empty>;
  const u = d.user,
    c = d.summary,
    a = c.allowance,
    self = session?.user?.id === id;
  return (
    <section className="sheet">
      <div className="stats-head">
        <h1>{u.name || u.email || id}</h1>
        <Link className="u" to="/admin/users">
          All users
        </Link>
      </div>
      <Row
        label={c.class + " access"}
        sub={c.enabled ? "Account enabled" : "Account disabled"}
      >
        <div className="v-list">
          <Line k="Source">
            {(
              {
                free: "Free account",
                subscription: "Confirmed payment",
                complimentary: "Complimentary access",
                staff: "Staff access",
                trial: "Trial",
              } as Record<string, string>
            )[c.source] ?? c.source}
            {c.accessUntil ? " · until " + datetime(c.accessUntil) : ""}
          </Line>
          <Line k="AI eligibility">
            <strong>{c.ai.eligible ? "Eligible" : "Blocked"}</strong> ·{" "}
            {c.ai.reason}
          </Line>
          <Line k="Generated today">
            {a.used.toLocaleString()} used + {a.reserved.toLocaleString()}{" "}
            reserved / {a.limit.toLocaleString()}
          </Line>
          <Line k="Resets">
            {new Date(a.resetsAt).toISOString().slice(0, 16).replace("T", " ")}{" "}
            UTC
          </Line>
          <Line k="Per run">
            {c.ai.maxOutputTokens.toLocaleString()} generated tokens · 1 active
            run per user
          </Line>
          <Line k="Saved workspaces">
            {c.workspaces.owned} / {c.workspaces.limit} ·{" "}
            {c.workspaces.membersPerInstance} people per workspace
          </Line>
        </div>
        <p className="muted small">
          AI allowance and saved-workspace limits are separate. Shared
          workspaces also require the owner’s access. Eligibility is checked
          again for each request.
        </p>
        <button
          className="btn btn-soft"
          disabled={self}
          onClick={() =>
            act(
              patch({ id, disabled: !u.disabled }).unwrap(),
              u.disabled ? "Account enabled" : "Account disabled",
            )
          }
        >
          {u.disabled ? "Enable account" : "Disable account"}
        </button>
      </Row>
      <Row
        label="Daily generated-token ceiling"
        sub={
          a.override === null
            ? "Class default"
            : "Custom ceiling · class default " + a.default.toLocaleString()
        }
      >
        <form
          className="inline-form"
          onSubmit={(e) => {
            e.preventDefault();
            void act(
              ceiling({ id, limit: Number(limit), reason: ceilingReason })
                .unwrap()
                .then((v) => {
                  setLimit("");
                  setCeilingReason("");
                  return v;
                }),
              "Daily ceiling saved",
            );
          }}
        >
          <input
            aria-label="Daily generated-token ceiling"
            type="number"
            min={0}
            max={100000000}
            required
            placeholder={String(a.limit)}
            value={limit}
            onChange={(e) => setLimit(e.target.value)}
          />
          <input
            aria-label="Ceiling reason"
            required
            placeholder="Reason"
            value={ceilingReason}
            onChange={(e) => setCeilingReason(e.target.value)}
          />
          <button className="btn btn-ink">Save ceiling</button>
          <button
            type="button"
            className="btn btn-soft"
            disabled={a.override === null}
            onClick={() =>
              act(
                ceiling({
                  id,
                  limit: null,
                  reason: "Reset to class default",
                })
                  .unwrap()
                  .then((v) => {
                    setLimit("");
                    setCeilingReason("");
                    return v;
                  }),
                "Class default restored",
              )
            }
          >
            Reset to default
          </button>
        </form>
        <p className="muted small">
          {limit !== ""
            ? "Sets an absolute ceiling of " +
              Number(limit).toLocaleString() +
              " generated tokens per UTC day. "
            : ""}
          Recorded usage stays unchanged. This does not change payment status,
          workspace limits or enabled state. Active runs keep their admitted
          budget.
        </p>
      </Row>
      <Row
        label="Complimentary Paying access"
        sub="Same limits as a paying customer; no Stripe charge"
      >
        <form
          className="inline-form"
          onSubmit={(e) => {
            e.preventDefault();
            void act(
              grant({ id, days: Number(days), reason }).unwrap(),
              "Complimentary Paying access granted",
            );
          }}
        >
          <input
            aria-label="Complimentary days"
            type="number"
            min={1}
            max={3660}
            value={days}
            onChange={(e) => setDays(e.target.value)}
          />
          <span>days</span>
          <input
            aria-label="Access reason"
            required
            placeholder="Reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
          <button className="btn btn-ink">Grant access</button>
          {(d.complimentary ?? []).some(
            (g: any) => !g.revokedAt && Date.parse(g.until) > Date.now(),
          ) && (
            <button
              type="button"
              className="btn btn-soft"
              onClick={() => act(endGrant(id).unwrap(), "Manual access ended")}
            >
              End manual access
            </button>
          )}
        </form>
        <p className="muted small">
          Does not create or cancel a subscription. A confirmed payment can
          still provide Paying access after a manual grant ends.
        </p>
      </Row>
      <details>
        <summary>Identity, payment contracts and administrative tools</summary>
        <Row label="Identity">
          <div className="v-list">
            <Line k="Email">
              {u.email} · {u.emailVerified ? "verified" : "unverified"}
            </Line>
            <Line k="User ID" mono>
              {u.id}
            </Line>
            <Line k="Identity provider" mono>
              {u.issuer} · {u.subject}
            </Line>
            <Line k="Sessions">{d.sessions}</Line>
          </div>
        </Row>
        <Row label="Staff permissions" sub="Separate from payments">
          <select
            aria-label="Staff permissions"
            value={u.role}
            disabled={self}
            onChange={(e) =>
              act(
                patch({ id, role: e.target.value }).unwrap(),
                "Permissions updated",
              )
            }
          >
            <option value="subscriber">User</option>
            <option value="creator">Creator</option>
            <option value="admin">Administrator</option>
          </select>
          <p className="muted small">
            Creators publish artifacts. Administrators also manage users and
            settings. Staff use Paying resource limits; this does not mean they
            have paid.
          </p>
        </Row>
        <Row label="Payment contracts">
          <div className="v-list">
            <Line k="Stripe customer" mono>
              {d.stripeCustomerId || "None"}
            </Line>
            {c.support.kind === "year" && c.support.active && (
              <Line k="Year funding">
                Paid through {datetime(c.support.until)}
              </Line>
            )}
            {(d.subscriptions ?? []).map((s: any) => (
              <Line key={s.id} k={s.id}>
                {s.status} · {s.cancelAtPeriodEnd ? "stops" : "renews"}{" "}
                {datetime(s.accessUntil)}
              </Line>
            ))}
            {(d.complimentary ?? []).map((g: any) => (
              <Line key={g.id} k="Complimentary">
                Until {datetime(g.until)} · {g.reason}{" "}
                {g.revokedAt ? "· revoked" : ""}
              </Line>
            ))}
          </div>
          <button
            className="btn btn-soft"
            onClick={() =>
              act(reconcile(id).unwrap(), "Payment refresh queued")
            }
          >
            Refresh payments
          </button>
        </Row>
        <Row label="Sessions">
          <button
            className="btn btn-soft"
            onClick={() =>
              act(revoke(id).unwrap(), "Sessions and live views revoked")
            }
          >
            Revoke sessions
          </button>
        </Row>
      </details>
      <Row label="Workspaces">
        <div className="v-list">
          {d.instances.map((i: any) => (
            <Line
              key={i.id}
              k={
                <Link className="u" to={"/admin/instances/" + i.id}>
                  {i.title}
                </Link>
              }
            >
              {i.role} · {i.state}
            </Line>
          ))}
        </div>
      </Row>
      <details>
        <summary>Recent model calls and audit history</summary>
        <Row label="Recent calls">
          <Calls calls={d.calls ?? []} />
        </Row>
        <Row label="Audit">
          <AuditList entries={d.audit ?? []} />
        </Row>
      </details>
    </section>
  );
}

function AdminInstances() {
  const [q, setQ] = useState("");
  const { data = [] } = api.useAdminInstancesQuery(q, {
    pollingInterval: 15000,
  });
  const [setState] = api.useSetInstanceStateMutation();
  const act = useAct();
  return (
    <section className="library">
      <div className="lib-head">
        <h2>Instances</h2>
        <div className="lib-search">
          <input
            placeholder="search title, owner, app …"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="Search instances"
          />
        </div>
      </div>
      <div className="lib-list">
        {data.map((i) => (
          <div
            className={"lib-row" + (i.state !== "active" ? " is-unlisted" : "")}
            key={i.id}
          >
            <div className="lib-main">
              <Link to={"/admin/instances/" + i.id}>
                <strong>{i.title}</strong>
              </Link>
              <span className="muted small">
                {i.artifactId} v{i.version} · {i.ownerName} · {i.kind}
              </span>
            </div>
            <span className="lib-tags">{i.members} members</span>
            <span className={"lib-status" + (i.activeViews ? " on" : "")}>
              {i.activeViews ? `${i.activeViews} live` : "idle"}
            </span>
            <span
              className={"lib-status" + (i.state === "active" ? "" : " bad")}
            >
              {i.state}
            </span>
            <div className="lib-actions">
              {i.state === "active" ? (
                <button
                  className="btn btn-soft"
                  onClick={() =>
                    confirm(
                      "Suspend this instance? Data is kept; live views close.",
                    ) &&
                    act(
                      setState({ id: i.id, action: "suspend" }).unwrap(),
                      "Instance suspended",
                    )
                  }
                >
                  Suspend
                </button>
              ) : (
                <button
                  className="btn btn-soft"
                  onClick={() =>
                    act(
                      setState({ id: i.id, action: "resume" }).unwrap(),
                      "Instance resumed",
                    )
                  }
                >
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
          <Line k="ID" mono>
            {i.id}
          </Line>
          <Line k="Origin" mono>
            {i.origin}
          </Line>
          <Line k="Release">
            {i.artifactId} v{i.version}{" "}
            <span className="mono muted">{i.releaseId}</span>
          </Line>
          <Line k="Owner">{i.ownerName}</Line>
          <Line k="Live views">{i.activeViews}</Line>
        </div>
        <p className="muted small">
          Administrators inspect metadata here; customer pages are never
          executed on the portal origin.
        </p>
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
            <Line
              key={m.userId}
              k={
                <Link className="u" to={"/admin/users/" + m.userId}>
                  {m.name || m.email}
                </Link>
              }
            >
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
        <span className="muted mono">
          {data.enabled ? data.priceId : "not configured"}
        </span>
      </div>
      <Tiles
        items={[
          [h.customers, "customers"],
          [h.activeSubscriptions, "active subscriptions"],
          [h.openCheckouts, "open checkouts"],
          [h.pending, "pending events"],
          [h.errors, "event errors"],
          [h.oldestPending ? datetime(h.oldestPending) : "—", "oldest pending"],
        ]}
      />
      <p className="muted small">
        Stripe is authoritative. Use the Stripe Dashboard or Customer Portal for
        refunds and plan changes; reconcile a user from their account page.
      </p>
      <div className="stats-block">
        <div className="views">
          {["", "pending", "processed", "error", "ignored"].map((s) => (
            <button
              key={s}
              className="tlink quiet"
              aria-pressed={state === s}
              onClick={() => setState(s)}
            >
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
                <button
                  className="tlink quiet"
                  onClick={() => act(retry(e.eventId).unwrap(), "Retry queued")}
                >
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

        <span className="snum">Tokens</span>
      </div>
      {calls.map((c) => (
        <div className="srow" key={c.id}>
          <span className="scol">{datetime(c.createdAt)}</span>
          <span className="sname mono">{c.modelUsed || c.model}</span>
          <span
            className={
              "scol" +
              (c.state === "failed" || c.state === "review" ? " warn" : "")
            }
          >
            {c.state}
            {c.errorCode ? " · " + c.errorCode : ""}
          </span>

          <span className="snum">
            {(c.inputTokens ?? 0) + (c.outputTokens ?? 0)}
          </span>
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
      <Tiles
        items={[
          [s.calls, "calls"],
          [s.activeUsers, "active users"],
          [s.inputTokens, "input tokens"],
          [s.outputTokens, "output tokens"],
          [s.cost ? "$" + s.cost.toFixed(4) : "—", "provider cost"],
        ]}
      />
      <p className="muted small">
        Provider cost may be zero or estimated for subscription/OAuth routes;
        Daily ceilings count generated tokens; input and cache usage are
        recorded separately.
      </p>
      <div className="lib-search">
        <input
          placeholder="filter by user id …"
          value={user}
          onChange={(e) => setUser(e.target.value)}
          aria-label="Filter by user"
        />
      </div>
      <div className="stats-block">
        <div className="srow shead">
          <span className="scol">When</span>
          <span className="scol">User</span>
          <span className="sname">Model</span>
          <span className="scol">State</span>

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
            <span
              className={
                "scol" +
                (c.state === "failed" || c.state === "review" ? " warn" : "")
              }
            >
              {c.state}
              {c.errorCode ? " · " + c.errorCode : ""}
            </span>

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

const thinkingLevels = [
  "off",
  "minimal",
  "low",
  "medium",
  "high",
  "xhigh",
  "max",
];

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
      const rev = await save({
        doc: body,
        revision: data.active.revision,
      }).unwrap();
      dispatch(show({ text: `Settings revision ${rev.revision} is active.` }));
    } catch (e) {
      setError(
        e instanceof SyntaxError
          ? "Settings JSON is invalid: " + e.message
          : errorMessage(e),
      );
    }
  };
  const approved = new Set(doc.models.allowed.map((m) => m.selector));
  const num = (v: string) => (v === "" ? 0 : Number(v));
  return (
    <section className="sheet">
      <div className="stats-head">
        <h1>Settings</h1>
        <span className="muted">
          revision {data.active.revision} ·{" "}
          {data.active.revision
            ? datetime(data.active.activatedAt)
            : "defaults"}
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
        <p className="muted small">
          Credentials live in deployment secret files and are never shown or
          edited here.
        </p>
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
          <textarea
            className="code-area"
            rows={28}
            spellCheck={false}
            value={rawText}
            onChange={(e) => setRawText(e.target.value)}
          />
        </Row>
      ) : (
        <>
          <Row
            label="Service controls"
            sub="apply to everyone, including open applications"
          >
            <label className="check">
              <input
                type="checkbox"
                checked={doc.operations.acceptNewVisits}
                onChange={(e) =>
                  upd((d) => {
                    d.operations.acceptNewVisits = e.target.checked;
                  })
                }
              />
              Accept new visits
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={doc.operations.samplingEnabled}
                onChange={(e) =>
                  upd((d) => {
                    d.operations.samplingEnabled = e.target.checked;
                  })
                }
              />
              Enable AI
            </label>
            <p className="muted small">
              Pausing AI stops new calls. Calls already running finish.
            </p>
            <label className="field">
              <span>Saved work</span>
              <select
                aria-label="Storage mode"
                value={doc.operations.storageMode}
                onChange={(e) =>
                  upd((d) => {
                    d.operations.storageMode = e.target
                      .value as SettingsDoc["operations"]["storageMode"];
                  })
                }
              >
                <option value="read-write">Read and write</option>
                <option value="read-only">Read only</option>
                <option value="off">Unavailable</option>
              </select>
            </label>
            <label className="field">
              <span>Notice (optional)</span>
              <textarea
                aria-label="Service notice"
                maxLength={500}
                rows={2}
                value={doc.operations.notice}
                onChange={(e) =>
                  upd((d) => {
                    d.operations.notice = e.target.value;
                  })
                }
              />
            </label>
          </Row>
          <Row
            label="Approved models"
            sub="from the operator's Pi configuration"
          >
            {data.providerModels.length === 0 && (
              <p className="warn">The model worker reports no models.</p>
            )}
            <div className="model-table">
              {data.providerModels.map((pm) => {
                const m = doc.models.allowed.find(
                  (a) => a.selector === pm.selector,
                );
                return (
                  <div
                    className={"model-line" + (m ? " on" : "")}
                    key={pm.selector}
                  >
                    <label className="check">
                      <input
                        type="checkbox"
                        checked={!!m}
                        onChange={(e) =>
                          upd((d) => {
                            if (e.target.checked)
                              d.models.allowed.push({
                                selector: pm.selector,
                                label: pm.name,
                                thinking:
                                  pm.thinkingLevels?.filter(
                                    (t) => t !== "off",
                                  ) ?? [],
                              });
                            else {
                              d.models.allowed = d.models.allowed.filter(
                                (a) => a.selector !== pm.selector,
                              );
                              for (const t of Object.keys(d.models.tiers))
                                if (d.models.tiers[t] === pm.selector)
                                  delete d.models.tiers[t];
                            }
                          })
                        }
                      />
                      <span className="mono">{pm.selector}</span>
                    </label>
                    {m && (
                      <span className="model-opts">
                        <input
                          aria-label="Label"
                          value={m.label}
                          onChange={(e) =>
                            upd(
                              (d) =>
                                (d.models.allowed.find(
                                  (a) => a.selector === pm.selector,
                                )!.label = e.target.value),
                            )
                          }
                        />

                        <span className="seg small">
                          {thinkingLevels
                            .filter(
                              (t) =>
                                (pm.thinkingLevels ?? []).includes(t) ||
                                m.thinking.includes(t),
                            )
                            .map((t) => (
                              <button
                                key={t}
                                aria-pressed={m.thinking.includes(t)}
                                onClick={() =>
                                  upd((d) => {
                                    const a = d.models.allowed.find(
                                      (x) => x.selector === pm.selector,
                                    )!;
                                    a.thinking = a.thinking.includes(t)
                                      ? a.thinking.filter((x) => x !== t)
                                      : [...a.thinking, t];
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
                  <select
                    value={doc.models.tiers[tier] ?? ""}
                    onChange={(e) =>
                      upd((d) =>
                        e.target.value
                          ? (d.models.tiers[tier] = e.target.value)
                          : delete d.models.tiers[tier],
                      )
                    }
                  >
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
          <Row
            label="Early access"
            sub="Paying customers get new artifacts first"
          >
            <NumField
              label="Default early-access days (0 = public immediately)"
              value={doc.publishing.earlyAccessDays}
              set={(v) =>
                upd((d) => {
                  d.publishing.earlyAccessDays = v;
                })
              }
            />
            <p className="muted small">
              Applies once, on an artifact’s first release. Later releases do
              not restart the period. Edit an artifact’s end date in Studio to
              shorten, extend or end it.
            </p>
          </Row>
          <Row label="Customer limits" sub="Free and Paying">
            <PlansEditor doc={doc} upd={upd} />
          </Row>
          <Row label="Demos" sub="visitors without an account">
            <label className="check">
              <input
                type="checkbox"
                checked={doc.demo.enabled}
                onChange={(e) =>
                  upd((d) => (d.demo.enabled = e.target.checked))
                }
              />{" "}
              Offer demos of artifacts that enable them
            </label>
            <div className="grid-fields">
              <NumField
                label="Demo lifetime (minutes)"
                value={doc.demo.ttlMinutes}
                set={(v) => upd((d) => (d.demo.ttlMinutes = v))}
              />
              <NumField
                label="Live demos (all visitors)"
                value={doc.demo.maxActive}
                set={(v) => upd((d) => (d.demo.maxActive = v))}
              />
              <NumField
                label="Demos per user per hour"
                value={doc.demo.perUserPerHour}
                set={(v) =>
                  upd((d) => {
                    d.demo.perUserPerHour = v;
                  })
                }
              />
              <NumField
                label="Demos per address per hour"
                value={doc.demo.perAddressPerHour}
                set={(v) => upd((d) => (d.demo.perAddressPerHour = v))}
              />
              <NumField
                label="Generated tokens per demo"
                value={doc.demo.tokens}
                set={(v) => upd((d) => (d.demo.tokens = v))}
              />
              <NumField
                label="Daily generated-token ceiling, all demos"
                value={doc.demo.dailyBudget}
                set={(v) => upd((d) => (d.demo.dailyBudget = v))}
              />
            </div>
          </Row>
          <Row label="Storefront" sub="public presentation">
            <label className="check">
              <input
                type="checkbox"
                checked={doc.storefront.journalEnabled}
                onChange={(e) =>
                  upd((d) => (d.storefront.journalEnabled = e.target.checked))
                }
              />{" "}
              Show the blog publicly
            </label>
            <p className="muted small">
              Off by default. Hides the journal pages, links and blog entries in
              RSS. Posts remain available in Studio.
            </p>
            <div className="grid-fields">
              <label className="field">
                <span>Title</span>
                <input
                  value={doc.storefront.title}
                  onChange={(e) =>
                    upd((d) => (d.storefront.title = e.target.value))
                  }
                />
              </label>
              <label className="field">
                <span>Tagline</span>
                <input
                  value={doc.storefront.tagline}
                  onChange={(e) =>
                    upd((d) => (d.storefront.tagline = e.target.value))
                  }
                />
              </label>
            </div>
            <label className="field">
              <span>Introduction (Markdown)</span>
              <textarea
                rows={5}
                value={doc.storefront.intro}
                onChange={(e) =>
                  upd((d) => (d.storefront.intro = e.target.value))
                }
              />
            </label>
          </Row>
          <Row label="Runtime" sub="concurrency, all users">
            <div className="grid-fields">
              <p>One active model run per user.</p>
              <NumField
                label="Concurrent calls globally"
                value={doc.allowance.globalConcurrency}
                set={(v) => upd((d) => (d.allowance.globalConcurrency = v))}
              />
              <NumField
                label="Generated tokens per run"
                value={doc.allowance.maxOutputTokens}
                set={(v) =>
                  upd((d) => {
                    d.allowance.maxOutputTokens = v;
                  })
                }
              />
              <NumField
                label="Max tool rounds"
                value={doc.allowance.maxToolRounds}
                set={(v) => upd((d) => (d.allowance.maxToolRounds = v))}
              />
            </div>
          </Row>
          <Row label="Limits" sub="hosted resources">
            <div className="grid-fields">
              <NumField
                label="Views per user"
                value={doc.limits.viewsPerUser}
                set={(v) =>
                  upd((d) => {
                    d.limits.viewsPerUser = v;
                  })
                }
              />
              <NumField
                label="Total saved MiB per instance"
                value={doc.limits.storageBytesPerInstance / (1 << 20)}
                set={(v) =>
                  upd((d) => {
                    d.limits.storageBytesPerInstance = Math.round(
                      v * (1 << 20),
                    );
                  })
                }
              />
              <NumField
                label="Documents per instance"
                value={doc.limits.documentsPerInstance}
                set={(v) => upd((d) => (d.limits.documentsPerInstance = v))}
              />
              <NumField
                label="Asset MiB per instance"
                value={Math.round(doc.limits.assetBytesPerInstance / (1 << 20))}
                set={(v) =>
                  upd((d) => (d.limits.assetBytesPerInstance = v * (1 << 20)))
                }
              />
              <NumField
                label="Max HTML KiB"
                value={Math.round(doc.limits.maxHtmlBytes / 1024)}
                set={(v) => upd((d) => (d.limits.maxHtmlBytes = v * 1024))}
              />
              <NumField
                label="Max media MiB"
                value={Math.round(doc.limits.maxMediaBytes / (1 << 20))}
                set={(v) =>
                  upd((d) => (d.limits.maxMediaBytes = v * (1 << 20)))
                }
              />
              <NumField
                label="Max attachment MiB"
                value={Math.round(doc.limits.maxAttachmentBytes / (1 << 20))}
                set={(v) =>
                  upd((d) => (d.limits.maxAttachmentBytes = v * (1 << 20)))
                }
              />
            </div>
          </Row>
          <Row label="Publishing">
            <label className="check">
              <input
                type="checkbox"
                checked={doc.publishing.allowPinnedAfterWithdraw}
                onChange={(e) =>
                  upd(
                    (d) =>
                      (d.publishing.allowPinnedAfterWithdraw =
                        e.target.checked),
                  )
                }
              />{" "}
              Instances pinned to a withdrawn release keep running
            </label>
          </Row>
        </>
      )}
      {error && <p className="notice bad">{error}</p>}
      <div className="form-foot sticky">
        <span className="muted small">
          Saving validates and activates a new revision atomically; calls record
          the revision they were admitted under.
        </span>
        <button className="btn btn-ink" onClick={submit} disabled={isLoading}>
          {isLoading ? "Saving…" : "Save & activate"}
        </button>
      </div>
      <Row label="History">
        <div className="v-list">
          {data.history.map((h) => (
            <Line key={h.revision} k={"revision " + h.revision}>
              {datetime(h.activatedAt)} · {h.document.models.allowed.length}{" "}
              models · {(h.document.plans ?? []).map((p) => p.name).join(", ")}
            </Line>
          ))}
          {data.history.length === 0 && (
            <span className="muted">Defaults only; no revision saved yet.</span>
          )}
        </div>
      </Row>
    </section>
  );
}

// Two fixed customer classes; no arbitrary product catalog.
function PlansEditor({
  doc,
  upd,
}: {
  doc: SettingsDoc;
  upd: (f: (d: SettingsDoc) => void) => void;
}) {
  return (
    <div className="plan-editor">
      {["free", "subscriber"].map((key) => {
        const p = doc.plans.find((p) => p.key === key)!;
        const edit = (f: (p: Plan) => void) =>
          upd((d) => f(d.plans.find((p) => p.key === key)!));
        return (
          <div key={key} className="plan-edit">
            <h3>{key === "free" ? "Free" : "Paying"}</h3>
            <div className="grid-fields">
              <NumField
                label="Generated tokens / UTC day"
                value={p.dailyTokens}
                set={(v) =>
                  edit((p) => {
                    p.dailyTokens = v;
                  })
                }
              />
              <NumField
                label="Saved workspaces"
                value={p.instances}
                set={(v) =>
                  edit((p) => {
                    p.instances = v;
                  })
                }
              />
              <NumField
                label="People per workspace"
                value={p.membersPerInstance}
                set={(v) =>
                  edit((p) => {
                    p.membersPerInstance = v;
                  })
                }
              />
            </div>
            {key === "subscriber" && (
              <details>
                <summary>Accepted Stripe price IDs</summary>
                <PriceField
                  initial={p.prices
                    .map((x) => [x.interval, x.priceId, x.label].join(" "))
                    .join("\n")}
                  set={(prices) =>
                    edit((p) => {
                      p.prices = prices;
                    })
                  }
                />
              </details>
            )}
          </div>
        );
      })}
      <p className="muted small">
        After early access ends, published artifacts are available to Free
        accounts. Payment grants the Paying limits at any contribution amount.
        Staff and complimentary access use those same limits.
      </p>
    </div>
  );
}

function PriceField({
  initial,
  set,
}: {
  initial: string;
  set: (p: Plan["prices"]) => void;
}) {
  const [text, setText] = useState(initial);
  return (
    <label className="field">
      <span>Stripe prices, one per line: month|year price_… label</span>
      <textarea
        className="slug"
        rows={2}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() =>
          set(
            text
              .split("\n")
              .map((l) => l.trim().split(/\s+/))
              .filter((w) => w.length >= 2)
              .map(([interval, priceId, ...label]) => ({
                interval: interval as "month" | "year",
                priceId,
                label: label.join(" "),
              })),
          )
        }
      />
    </label>
  );
}

function NumField({
  label,
  value,
  set,
}: {
  label: string;
  value: number;
  set: (v: number) => void;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <input
        type="number"
        value={value}
        onChange={(e) =>
          set(e.target.value === "" ? 0 : Number(e.target.value))
        }
      />
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
          <span className="scol mono muted">
            {e.actorId ? short(e.actorId, 14) : "operator"}
          </span>
          <span className="sname mono muted">
            {JSON.stringify(e.summary) === "{}"
              ? ""
              : JSON.stringify(e.summary)}
          </span>
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
          <input
            placeholder="subject id …"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            aria-label="Filter by subject"
          />
        </div>
      </div>
      <AuditList entries={data} />
    </section>
  );
}
