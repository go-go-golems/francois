import React, { useEffect } from "react";
import { useDispatch, useSelector } from "react-redux";
import { api, errorMessage } from "./api";
import { clear, navigate, show, State } from "./store";

// SPA routes; every other path (the storefront) is rendered by the server
// and is reached with a normal navigation.
const spaRoutes = ["/instances", "/account", "/studio", "/admin"];
const inSPA = (to: string) =>
  spaRoutes.some(
    (p) => to === p || to.startsWith(p + "/") || to.startsWith(p + "?"),
  );

export function Link(
  props: React.AnchorHTMLAttributes<HTMLAnchorElement> & { to: string },
) {
  const { to, onClick, ...rest } = props;
  return (
    <a
      {...rest}
      href={to}
      onClick={(e) => {
        onClick?.(e);
        if (
          !inSPA(to) ||
          e.defaultPrevented ||
          e.metaKey ||
          e.ctrlKey ||
          e.shiftKey ||
          e.button !== 0
        )
          return;
        e.preventDefault();
        navigate(to);
      }}
    />
  );
}

export function useRoute() {
  return useSelector((s: State) => s.route);
}

export const loginHref = (ret = location.pathname + location.search) =>
  "/signin?return=" + encodeURIComponent(ret);

export function Header() {
  const { data: session } = api.useSessionQuery();
  const signedIn = session?.authenticated && session.user?.role !== "guest";
  return (
    <header className="site-header page">
      <Link className="site-wordmark" to="/">
        {session?.brand ?? "slop.boutique"}
      </Link>
      <p className="site-statement">
        A lab for computation and intelligence as material and medium. Run by
        Manuel and Ramon.
      </p>
      <nav className="site-nav" aria-label="Main navigation">
        {signedIn ? (
          <Link to="/account">Account</Link>
        ) : (
          session?.loginAvailable && <a href={loginHref()}>Sign in</a>
        )}
        <a className="site-support" href="/pricing">
          <span className="support-desktop">Support the lab</span>
          <span className="support-mobile">Support</span>
        </a>
      </nav>
    </header>
  );
}

export function Notice() {
  const n = useSelector((s: State) => s.notice);
  const dispatch = useDispatch();
  useEffect(() => {
    if (!n.text) return;
    const t = setTimeout(() => dispatch(clear()), 6000);
    return () => clearTimeout(t);
  }, [n.text]);
  if (!n.text) return null;
  return (
    <div
      className={"toast" + (n.bad ? " bad" : "")}
      role="status"
      onClick={() => dispatch(clear())}
    >
      {n.text}
    </div>
  );
}

// The React shell uses the same structure and classes as the public templates.
export function Footer() {
  const { data: session } = api.useSessionQuery();
  const signedIn = session?.authenticated && session.user?.role !== "guest";
  const staff =
    session?.user?.role === "admin" || session?.user?.role === "creator";
  return (
    <footer className="site-footer">
      <div className="footer-brand">
        <Link className="site-wordmark" to="/">
          {session?.brand ?? "slop.boutique"}
        </Link>
        <a href="mailto:hej@slop.boutique">hej@slop.boutique</a>
      </div>
      <div>
        <span>For humans</span>
        <a href="/feed.xml">RSS</a>
        {session?.journalEnabled && <a href="/journal">Journal</a>}
        <a href="/pricing">Support the lab</a>
        {signedIn && <Link to="/account">Account</Link>}
        {staff && <Link to="/studio">Studio</Link>}
        {session?.user?.role === "admin" && <Link to="/admin">Admin</Link>}
      </div>
      <div>
        <span>Information</span>
        <a href="/privacy">Privacy</a>
        <a href="/terms">Terms</a>
      </div>
      <div className="footer-type">
        <span>Type</span>
        {session?.typeCredit && (
          <p>
            Set in <a href="https://abcdinamo.com/free/areal">ABC Areal</a>, by{" "}
            <a href="https://abcdinamo.com">ABC Dinamo</a>
          </p>
        )}
      </div>
    </footer>
  );
}

// Row renders the label-column layout: a grey label column, then content.
export function Row({
  label,
  sub,
  children,
  ruled = true,
}: {
  label: React.ReactNode;
  sub?: React.ReactNode;
  children: React.ReactNode;
  ruled?: boolean;
}) {
  return (
    <section className={"row section" + (ruled ? " ruled" : "")}>
      <div className="row-label">
        <span>{label}</span>
        {sub && <span>{sub}</span>}
      </div>
      <div className="row-body">{children}</div>
    </section>
  );
}

export function Line({
  k,
  children,
  mono,
}: {
  k: React.ReactNode;
  children: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="kv">
      <span>{k}</span>
      <span className={mono ? "mono" : undefined}>{children}</span>
    </div>
  );
}

export const date = (s?: string) =>
  s
    ? new Date(s).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      })
    : "—";
export const dateUTC = (s?: string) =>
  s
    ? new Date(s).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
        timeZone: "UTC",
      }) + " UTC"
    : "—";
export const datetime = (s?: string) =>
  s
    ? new Date(s).toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })
    : "—";
export const bytes = (n: number) =>
  n < 1024
    ? n + " B"
    : n < 1 << 20
      ? (n / 1024).toFixed(1) + " KB"
      : (n / (1 << 20)).toFixed(1) + " MB";
export const short = (s: string, n = 10) =>
  s.length > n ? s.slice(0, n) + "…" : s;

const reasons: Record<string, string> = {
  subscriber: "Active subscription",
  subscriber_trial: "Trial",
  complimentary: "Complimentary access",
  account_disabled: "Account disabled",
  billing_conflict: "Billing needs review",
  payment_past_due: "Payment past due",
  payment_incomplete: "Payment incomplete",
  subscription_required: "No subscription",
  subscription_expired: "Subscription expired",
};
export const reasonLabel = (r?: string) =>
  r ? (reasons[r] ?? r.replace(/_/g, " ")) : "—";

export function Empty({ children }: { children: React.ReactNode }) {
  return <p className="empty">{children}</p>;
}

// Monogram is the typographic stand-in for screenshots.
export function Monogram({ title, caps }: { title: string; caps: string[] }) {
  const letters = title
    .split(/\s+/)
    .map((w) => w[0])
    .join("")
    .slice(0, 3)
    .toUpperCase();
  return (
    <div className="mono-thumb" aria-hidden="true">
      <span className="mono-letters">{letters}</span>
      <span className="mono-caps">
        {caps.filter((c) => c !== "downloads").join(" · ")}
      </span>
    </div>
  );
}

// useAct runs a mutation and reports success or failure as a toast.
export function useAct() {
  const dispatch = useDispatch();
  return async <T,>(p: Promise<T>, ok?: string): Promise<T | undefined> => {
    try {
      const v = await p;
      if (ok) dispatch(show({ text: ok }));
      return v;
    } catch (e) {
      dispatch(show({ text: errorMessage(e), bad: true }));
      return undefined;
    }
  };
}
