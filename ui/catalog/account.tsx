import React, { useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { api, errorMessage } from "./api";
import { navigate, show } from "./store";
import { ApiTokens } from "./tokens";
import { RecurringSupport } from "./recurring-support";
import { Empty, loginHref, Row, SignOut } from "./ui";

export function Account() {
  const { data: session } = api.useSessionQuery();
  useEffect(() => {
    if (!session?.brand) return;
    const previous = document.title;
    document.title = `Account · ${session.brand}`;
    return () => {
      document.title = previous;
    };
  }, [session?.brand]);
  const params = new URLSearchParams(location.search);
  const returned = params.get("checkout");
  const [polling, setPolling] = useState(!!returned);
  const { data: a, isLoading } = api.useAccountQuery(undefined, {
    skip: !session?.authenticated,
    pollingInterval: polling ? 2500 : 0,
  });
  const returnedAttempt = a?.checkouts.find(
    (checkout) => checkout.sessionId === returned,
  );
  const returnedConfirmed =
    !!returnedAttempt &&
    (returnedAttempt.price === "support:year:99999999"
      ? !!returnedAttempt.supportUntil
      : !!a?.support.active);
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
    if (polling && returnedConfirmed) {
      setPolling(false);
      dispatch(api.util.invalidateTags(["Session"]));
      dispatch(show({ text: "Thank you for supporting the lab." }));
    }
  }, [polling, returnedConfirmed]);
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
      <section className="final-section">
        <div className="final-label">
          Account<p>{a.user.email}</p>
        </div>
        <div className="final-content">
          <h1>
            {a.support.active
              ? `Thank you, ${a.user.name || "friend"}.`
              : "Your account."}
          </h1>
          {a.support.active && (
            <div className="support-impact">
              {a.support.startedAt ? (
                <>
                  <span className="support-impact-count">
                    {a.support.experiments}
                  </span>
                  <div>
                    <p>
                      experiments made possible since you started supporting us
                      on{" "}
                      {new Date(a.support.startedAt).toLocaleDateString(
                        "en-GB",
                        { day: "numeric", month: "long", year: "numeric" },
                      )}
                      .
                    </p>
                    <a className="u" href="/">
                      See them in the archive
                    </a>
                  </div>
                </>
              ) : (
                <p>Your support keeps the lab running. Thank you.</p>
              )}
            </div>
          )}
          <div className="account-detail">
            <span>Email</span>
            <p>{a.user.email}</p>
          </div>
          <div className="account-detail">
            <span>{a.support.active ? "Support" : "Access"}</span>
            <p>
              {a.support.active && a.support.amountCents > 0
                ? a.support.kind === "year"
                  ? `${(a.support.amountCents / 100).toLocaleString("en-US", { minimumFractionDigits: 2 })} USD. A year funded.`
                  : `${a.support.tier === "enterprise" ? "Enterprise support, " : ""}${a.support.amountCents / 100} USD a month.`
                : a.entitlement.plan.name}
            </p>
          </div>
          {a.support.active && a.support.until && (
            <p className="small muted">
              {a.support.cancelScheduled
                ? "Support continues until "
                : "Next payment on "}
              {new Date(a.support.until).toLocaleDateString("en-GB", {
                day: "numeric",
                month: "long",
                year: "numeric",
              })}
              .
            </p>
          )}
          {a.support.active && (
            <p>
              {a.support.kind === "year"
                ? "Thank you for funding a year. This year payment will not renew automatically."
                : "You can stop any time. There will be no further payments, and everything stays free and open to you."}
            </p>
          )}
          {a.support.kind === "year" && (
            <RecurringSupport subscriptions={a.subscriptions} />
          )}
          {polling && !returnedConfirmed && (
            <p role="status">Checking your subscription…</p>
          )}
          {returned && !polling && !returnedConfirmed && (
            <p role="status">
              Your payment is still being confirmed. You can check again by
              reloading this page or manage your billing below.
            </p>
          )}
          {a.user.disabled && (
            <p>
              This account is disabled. You can still manage your subscription.
            </p>
          )}
          {a.billingEnabled && a.hasCustomer && (
            <p>
              <button className="tlink" onClick={manage} disabled={opening}>
                Manage subscription and receipts
              </button>
            </p>
          )}
          <p>
            <SignOut />
          </p>
        </div>
      </section>
      {!a.support.active && a.entitlement.source === "free" && (
        <aside
          className="support-callout"
          aria-labelledby="account-support-heading"
        >
          <p className="callout-label">Support the lab</p>
          <div>
            <h2 id="account-support-heading">Keep the lab running.</h2>
            <p>
              We prototype and explore, and share the work here. The lab is
              funded by people who want it to keep going. No ads, no clients, no
              investors.
            </p>
            <a className="site-button callout-button" href="/pricing">
              Support the lab →
            </a>
            {session.contact && (
              <a
                className="u callout-contact"
                href={`mailto:${session.contact}`}
              >
                Or get in touch
              </a>
            )}
          </div>
        </aside>
      )}
      {session.user &&
        (session.user.role === "admin" || session.user.role === "creator") && (
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
