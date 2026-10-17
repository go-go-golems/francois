import React, { useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { api, errorMessage } from "./api";
import { navigate, show } from "./store";
import { ApiTokens } from "./tokens";
import { Empty, loginHref, Row } from "./ui";

export function Account() {
  const { data: session } = api.useSessionQuery();
  useEffect(() => { if (session?.brand) document.title = `Account · ${session.brand}`; }, [session?.brand]);
  const params = new URLSearchParams(location.search);
  const returned = params.get("checkout");
  const [polling, setPolling] = useState(!!returned);
  const { data: a, isLoading } = api.useAccountQuery(undefined, {
    skip: !session?.authenticated,
    pollingInterval: polling ? 2500 : 0,
  });
  const [portal, { isLoading: opening }] = api.usePortalMutation();
  const [refresh] = api.useRefreshBillingMutation();
  const [logout] = api.useLogoutMutation();
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
  if (!session)
    return (
      <main>
        <Empty>Loading…</Empty>
      </main>
    );
  if (!session.authenticated)
    return (
      <main className="lost">
        <h1>Sign in to see your account.</h1>
        <a className="btn btn-ink" href={loginHref("/account")}>
          Sign in
        </a>
      </main>
    );
  if (isLoading || !a)
    return (
      <main>
        <Empty>Loading…</Empty>
      </main>
    );
  const manage = async () => {
    try {
      const { url } = await portal().unwrap();
      location.href = url;
    } catch (e) {
      dispatch(show({ text: errorMessage(e), bad: true }));
    }
  };
  return (
    <main className="final-account">
      <section className="final-section"><div className="final-label">Account<p>{a.user.email}</p></div><div className="final-content">
      <h1>{a.entitlement.source === "subscription" || a.entitlement.source === "trial" ? `Thank you, ${a.user.name || "friend"}.` : "Your account."}</h1>
      <div className="account-detail"><span>Email</span><p>{a.user.email}</p></div>
      <div className="account-detail"><span>Support</span><p>{a.entitlement.plan.name}</p></div>
      {polling && !a.access.allowed && <p role="status">Checking your subscription…</p>}
      {a.user.disabled && <p>This account is disabled. You can still manage your subscription.</p>}
      {a.billingEnabled && a.hasCustomer && (
        <p>
          <button className="tlink" onClick={manage} disabled={opening}>
            Manage subscription
          </button>
        </p>
      )}
      <p>
        <button
          className="tlink"
          onClick={async () => {
            try {
              const out = await logout().unwrap();
              location.assign(out.redirect || "/");
            } catch (e) {
              dispatch(show({ text: errorMessage(e), bad: true }));
            }
          }}
        >
          Sign out
        </button>
      </p>
      </div></section>
      {session.user && (session.user.role === "admin" || session.user.role === "creator") && (
        <Row label="API tokens">
          <ApiTokens role={session.user.role} />
        </Row>
      )}
    </main>
  );
}

export function goAccount() {
  navigate("/account");
}
