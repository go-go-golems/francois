import React, { useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { api, errorCode, errorMessage } from "./api";
import { navigate, show } from "./store";
import { date, datetime, Empty, Line, Link, loginHref, reasonLabel, Row } from "./ui";

export function Account() {
  const { data: session } = api.useSessionQuery();
  const params = new URLSearchParams(location.search);
  const returned = params.get("checkout");
  const [polling, setPolling] = useState(!!returned);
  const { data: a, isLoading, refetch } = api.useAccountQuery(undefined, {
    skip: !session?.authenticated,
    pollingInterval: polling ? 2500 : 0,
  });
  const [checkout, { isLoading: starting }] = api.useCheckoutMutation();
  const [portal, { isLoading: opening }] = api.usePortalMutation();
  const [refresh] = api.useRefreshBillingMutation();
  const dispatch = useDispatch();
  // A Checkout return is advisory: ask for reconciliation and poll the
  // projection for a bounded time; never treat the URL as proof of payment.
  useEffect(() => {
    if (!returned || !session?.authenticated) return;
    void refresh(returned);
    const stop = setTimeout(() => setPolling(false), 60000);
    return () => clearTimeout(stop);
  }, [returned, session?.authenticated]);
  useEffect(() => {
    if (polling && a?.access.allowed) {
      setPolling(false);
      dispatch(api.util.invalidateTags(["Session"]));
      dispatch(show({ text: "Your subscription is active." }));
    }
  }, [polling, a?.access.allowed]);
  if (!session) return <main><Empty>Loading…</Empty></main>;
  if (!session.authenticated)
    return (
      <main className="lost">
        <h1>Sign in to see your account.</h1>
        <a className="btn btn-ink" href={loginHref("/account")}>
          Sign in
        </a>
      </main>
    );
  if (isLoading || !a) return <main><Empty>Loading…</Empty></main>;
  const start = async () => {
    try {
      const { url } = await checkout().unwrap();
      location.href = url;
    } catch (e) {
      if (errorCode(e) === "subscription_open") return manage();
      dispatch(show({ text: errorMessage(e), bad: true }));
    }
  };
  const manage = async () => {
    try {
      const { url } = await portal().unwrap();
      location.href = url;
    } catch (e) {
      dispatch(show({ text: errorMessage(e), bad: true }));
    }
  };
  const sub = a.subscriptions[0];
  const comp = a.complimentary.find((g) => !g.revokedAt && new Date(g.until) > new Date());
  const pct = a.allowance.included + a.allowance.adjustments > 0 ? Math.min(100, ((a.allowance.used + a.allowance.reserved) / (a.allowance.included + a.allowance.adjustments)) * 100) : 0;
  return (
    <main className="sheet">
      <section className="stats-head">
        <h1>Account</h1>
        <span className={"status " + (a.access.allowed ? "on" : "off")}>{reasonLabel(a.access.reason)}</span>
      </section>
      {polling && !a.access.allowed && <p className="notice">Checking your subscription status… Activation can take a few seconds after payment.</p>}
      {a.user.disabled && <p className="notice bad">This account is disabled. You can still manage or cancel billing.</p>}
      <Row label="Profile" sub="from your identity provider">
        <div className="v-list">
          <Line k="Name">{a.user.name || "—"}</Line>
          <Line k="Email">
            {a.user.email || "—"} {a.user.emailVerified ? <span className="muted">verified</span> : <span className="warn">not verified</span>}
          </Line>
          <Line k="Role">{a.user.role}</Line>
          <Line k="Member since">{date(a.user.createdAt)}</Line>
          <Line k="Active sessions">{a.sessions}</Line>
        </div>
        <p className="muted small">Password, email verification and MFA are managed by the identity provider.</p>
      </Row>
      <Row label="Membership" sub="hosted instances, models, downloads">
        <div className="v-list">
          <Line k="Access">{a.access.allowed ? "Allowed" : "Not active"} · {reasonLabel(a.access.reason)}</Line>
          {a.access.until && (
            <Line k={a.access.cancelScheduled ? "Ends" : "Renews"}>
              {date(a.access.until)}
              {a.access.cancelScheduled && <span className="muted"> · cancellation scheduled</span>}
            </Line>
          )}
          {sub && <Line k="Stripe status" mono>{sub.status}</Line>}
          {comp && <Line k="Complimentary until">{date(comp.until)} <span className="muted">· {comp.reason}</span></Line>}
        </div>
        <div className="actions">
          {!a.billingEnabled ? (
            <span className="muted">Billing is not configured on this server.</span>
          ) : a.hasCustomer && sub ? (
            <button className="btn btn-ink" onClick={manage} disabled={opening}>
              Manage billing
            </button>
          ) : (
            !a.user.disabled && (
              <button className="btn btn-ink" onClick={start} disabled={starting || !a.user.emailVerified}>
                {starting ? "Opening Checkout…" : "Subscribe"}
              </button>
            )
          )}
          {a.hasCustomer && !sub && (
            <button className="btn btn-soft" onClick={manage} disabled={opening}>
              Billing history
            </button>
          )}
          {a.billingEnabled && a.hasCustomer && (
            <button
              className="tlink quiet"
              onClick={async () => {
                await refresh();
                setPolling(true);
                setTimeout(() => setPolling(false), 20000);
                refetch();
              }}
            >
              Refresh status
            </button>
          )}
        </div>
        {!a.user.emailVerified && a.billingEnabled && <p className="warn small">Verify your email with the identity provider before subscribing.</p>}
      </Row>
      <Row label="Model allowance" sub={`period ${a.allowance.period} · resets ${date(a.allowance.resetsAt)}`}>
        <div className="meter" aria-label="Allowance used">
          <div style={{ width: pct + "%" }} />
        </div>
        <div className="stat-tiles three">
          <div className="stat-tile">
            <span className="stat-n">{a.allowance.remaining}</span>
            <span className="stat-label">credits remaining</span>
          </div>
          <div className="stat-tile">
            <span className="stat-n">{a.allowance.used}</span>
            <span className="stat-label">used this period</span>
          </div>
          <div className="stat-tile">
            <span className="stat-n">{a.allowance.included + a.allowance.adjustments}</span>
            <span className="stat-label">included{a.allowance.adjustments ? ` (incl. ${a.allowance.adjustments} adjusted)` : ""}</span>
          </div>
        </div>
        {a.allowance.daily > 0 && (
          <p className="muted small">
            Today: {a.allowance.usedToday} of {a.allowance.daily} daily credits. Started calls are charged even if stopped.
          </p>
        )}
        {a.models.length > 0 && (
          <div className="v-list">
            {a.models.map((m) => (
              <Line key={m.selector} k={m.label || m.selector}>
                <span className="mono">{m.selector}</span> · {m.credits} {m.credits === 1 ? "credit" : "credits"} per call
              </Line>
            ))}
          </div>
        )}
      </Row>
      <Row label="Instances">
        <p>
          <Link className="u" to="/instances/">
            Your instances
          </Link>{" "}
          <span className="muted">— private by default; share them with other subscribers explicitly.</span>
        </p>
      </Row>
      {a.checkouts.length > 0 && (
        <Row label="Checkout attempts">
          <div className="v-list">
            {a.checkouts.map((c) => (
              <Line key={c.id} k={datetime(c.createdAt)} mono>
                {c.state}
              </Line>
            ))}
          </div>
        </Row>
      )}
    </main>
  );
}

export function goAccount() {
  navigate("/account");
}
