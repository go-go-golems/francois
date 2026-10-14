import React, { useEffect } from "react";
import { useDispatch, useSelector } from "react-redux";
import { api } from "./api";
import { clear, navigate, State } from "./store";

// SPA routes; every other path (the storefront) is rendered by the server
// and is reached with a normal navigation.
const spaRoutes = ["/instances", "/account", "/studio", "/admin"];
const inSPA = (to: string) => spaRoutes.some((p) => to === p || to.startsWith(p + "/") || to.startsWith(p + "?"));

export function Link(props: React.AnchorHTMLAttributes<HTMLAnchorElement> & { to: string }) {
  const { to, onClick, ...rest } = props;
  return (
    <a
      {...rest}
      href={to}
      onClick={(e) => {
        onClick?.(e);
        if (!inSPA(to) || e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        e.preventDefault();
        navigate(to);
      }}
    />
  );
}

export function useRoute() {
  return useSelector((s: State) => s.route);
}

export const loginHref = (ret = location.pathname + location.search) => "/auth/login?return=" + encodeURIComponent(ret);

export function Brand() {
  return (
    <svg width="18" height="18" viewBox="0 0 32 32" aria-hidden="true">
      <rect width="32" height="32" fill="#111" />
      <rect x="9" y="7" width="5" height="18" fill="#fff" />
      <rect x="9" y="7" width="14" height="5" fill="#fff" />
      <rect x="9" y="15" width="10" height="4" fill="#fff" />
    </svg>
  );
}

export function Header() {
  const { data: session } = api.useSessionQuery();
  const [logout] = api.useLogoutMutation();
  const { path } = useRoute();
  const nav: [string, string][] = [
    ["/apps", "Catalog"],
    ["/journal", "Journal"],
  ];
  if (session?.authenticated) nav.push(["/instances/", "Instances"], ["/account", "Account"]);
  if (session?.user?.role === "creator" || session?.user?.role === "admin") nav.push(["/studio", "Studio"]);
  if (session?.user?.role === "admin") nav.push(["/admin", "Admin"]);
  const current = (to: string) => path.startsWith(to.replace(/\/$/, ""));
  const crumb = path.split("/").filter(Boolean)[0] ?? "";
  return (
    <header className="bar">
      <div className="page bar-inner">
        <Link className="brand" to="/" title="Francois">
          <Brand />
        </Link>
        <div className="addr">
          <Link className="addr-host" to="/">
            francois/
          </Link>
          <span className="addr-path">{crumb}</span>
        </div>
        <nav className="nav">
          {nav.map(([to, label]) => (
            <Link key={to} to={to} aria-current={current(to) ? "page" : undefined}>
              {label}
            </Link>
          ))}
          {session?.authenticated ? (
            <button
              className="tlink quiet"
              onClick={async () => {
                const out = await logout();
                // Leave through the identity provider so its session ends too;
                // it returns the browser to the portal afterwards.
                if ("data" in out && out.data?.redirect) window.location.assign(out.data.redirect);
                else location.assign("/");
              }}
            >
              Sign out
            </button>
          ) : (
            session?.loginAvailable && (
              <a className="tlink" href={loginHref()}>
                Sign in
              </a>
            )
          )}
        </nav>
      </div>
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
    <div className={"toast" + (n.bad ? " bad" : "")} role="status" onClick={() => dispatch(clear())}>
      {n.text}
    </div>
  );
}

export function Footer() {
  return (
    <footer className="foot">
      <div className="foot-half">
        <h2>For people</h2>
        <div className="foot-cols">
          <div>
            <span>Here</span>
            <a href="/apps">Catalog</a>
            <a href="/journal">Journal</a>
            <Link to="/instances/">Instances</Link>
            <Link to="/account">Account</Link>
          </div>
          <div>
            <span>Legal</span>
            <a href="/privacy">Privacy</a>
            <a href="/terms">Terms</a>
          </div>
          <div>
            <span>Notes</span>
            <span className="foot-note">
              Applications run on their own origins.
              <br />
              Your data stays in your instance.
            </span>
          </div>
        </div>
      </div>
      <div className="foot-half">
        <h2>For machines</h2>
        <div className="foot-cols">
          <div>
            <span>Start</span>
            <a className="path" href="/api/catalog">
              /api/catalog
            </a>
            <a className="path" href="/healthz">
              /healthz
            </a>
          </div>
          <div>
            <span>Runtime</span>
            <span className="foot-note">
              window.claude: db, room,
              <br />
              user, sample, assets
            </span>
          </div>
        </div>
      </div>
    </footer>
  );
}

// Row renders the label-column layout: a grey label column, then content.
export function Row({ label, sub, children, ruled = true }: { label: React.ReactNode; sub?: React.ReactNode; children: React.ReactNode; ruled?: boolean }) {
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

export function Line({ k, children, mono }: { k: React.ReactNode; children: React.ReactNode; mono?: boolean }) {
  return (
    <div className="kv">
      <span>{k}</span>
      <span className={mono ? "mono" : undefined}>{children}</span>
    </div>
  );
}

export const date = (s?: string) => (s ? new Date(s).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }) : "—");
export const dateUTC = (s?: string) => (s ? new Date(s).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" }) + " UTC" : "—");
export const datetime = (s?: string) => (s ? new Date(s).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }) : "—");
export const bytes = (n: number) => (n < 1024 ? n + " B" : n < 1 << 20 ? (n / 1024).toFixed(1) + " KB" : (n / (1 << 20)).toFixed(1) + " MB");
export const short = (s: string, n = 10) => (s.length > n ? s.slice(0, n) + "…" : s);

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
export const reasonLabel = (r?: string) => (r ? (reasons[r] ?? r.replace(/_/g, " ")) : "—");

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
      <span className="mono-caps">{caps.filter((c) => c !== "downloads").join(" · ")}</span>
    </div>
  );
}
