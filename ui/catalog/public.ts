import { serviceNotice } from "./service-notice";
// public.ts adds the session-dependent actions to server-rendered storefront
// pages: sign out, run an artifact, try a demo. Pages are complete without
// it; it only replaces placeholders. No framework, to stay small.

type Plan = {
  key: string;
  name: string;
  rank: number;
  source: string;
  paid: boolean;
  instances: number;
};
type Session = {
  authenticated: boolean;
  loginAvailable: boolean;
  csrfToken?: string;
  user?: { role: string; disabled: boolean; emailVerified: boolean };
  plan?: Plan;
};

class RequestError extends Error {
  constructor(
    message: string,
    readonly code: string,
  ) {
    super(message);
  }
}

type Failure = { error?: { code?: string; message?: string } };

let session: Promise<Session> | undefined;
const getSession = () => (session ??= fetch("/api/session", { credentials: "same-origin" }).then((r) => r.json() as Promise<Session>));

async function post<T>(path: string, body: unknown): Promise<T> {
  const s = await getSession();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (s.csrfToken) headers["X-CSRF-Token"] = s.csrfToken;
  const r = await fetch(path, {
    method: "POST",
    credentials: "same-origin",
    headers,
    body: JSON.stringify(body),
  });
  const out = (await r.json().catch(() => ({}))) as T & Failure;
  if (!r.ok) throw new RequestError(out.error?.message || "Request failed", out.error?.code || "");
  return out;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, attrs: Record<string, string>, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  if (text) e.textContent = text;
  return e;
}

const loginHref = () => "/auth/login?return=" + encodeURIComponent(location.pathname + location.search);

function fail(box: HTMLElement, message: string) {
  box.querySelector(".action-error")?.remove();
  const notice = serviceNotice(message);
  notice.classList.add("action-error");
  box.append(notice);
}

async function renderActions(box: HTMLElement) {
  const s = await getSession();
  const { release, title, demo } = box.dataset;
  const meta = box.querySelector(".muted");
  box.replaceChildren();
  const required = Number(box.dataset.rank ?? "1");
  const plan = s.plan;
  // Mirrors Entitlement.CanRun plus the instance allowance; the server decides.
  const canRun = !!plan && (plan.source === "staff" || (plan.source !== "guest" && plan.source !== "disabled" && plan.rank >= required && plan.instances > 0));
  // One black button per screen: running when the plan allows it, otherwise
  // the demo when there is one; everything else is a text link.
  const hasDemo = demo === "true";
  const demoAction = (primary: boolean) => {
    const btn = el("button", { class: primary ? "btn btn-ink" : "tlink", type: "button" }, "Try");
    btn.addEventListener("click", async () => {
      btn.setAttribute("disabled", "");
      try {
        const inst = await post<{ id: string }>("/api/demos", {
          artifactId: box.dataset.artifact,
        });
        location.assign("/instances/" + inst.id);
      } catch (e) {
        btn.removeAttribute("disabled");
        fail(box, (e as Error).message);
      }
    });
    return btn;
  };
  if (release && canRun) {
    const run = el("button", { class: "btn btn-ink", type: "button" }, "Open");
    run.addEventListener("click", async () => {
      run.setAttribute("disabled", "");
      try {
        const inst = await post<{ id: string }>("/api/instances/open", {
          releaseId: release,
          title,
        });
        location.assign("/instances/" + inst.id);
      } catch (e) {
        run.removeAttribute("disabled");
        fail(box, (e as Error).message);
      }
    });
    box.append(run);
  } else {
    if (hasDemo) box.append(demoAction(true));
    if (release && !hasDemo) {
      const cls = hasDemo ? "tlink" : "btn btn-ink";
      const name = box.dataset.plan;
      box.append(el("a", { class: cls, href: loginHref() }, "Sign in"));
    }
  }
}

// bindCheckout turns the pricing page's subscribe links into Checkout for
// signed-in visitors; an existing subscription opens the billing portal,
// where plans are changed.
async function bindCheckout(box: HTMLElement) {
  const s = await getSession();
  if (!s.authenticated) return;
  box.querySelectorAll<HTMLAnchorElement>("a[data-interval]").forEach((a) => {
    a.addEventListener("click", async (e) => {
      e.preventDefault();
      a.setAttribute("aria-busy", "true");
      try {
        const out = await post<{ url: string }>("/api/billing/checkout", {
          plan: box.dataset.plan,
          interval: a.dataset.interval,
        });
        location.assign(out.url);
      } catch (err) {
        if (err instanceof RequestError && err.code === "subscription_open") {
          try {
            const out = await post<{ url: string }>("/api/billing/portal", {});
            location.assign(out.url);
            return;
          } catch (perr) {
            err = perr;
          }
        }
        a.removeAttribute("aria-busy");
        fail(box, (err as Error).message);
      }
    });
  });
}

function bindSignOut(btn: HTMLElement) {
  btn.addEventListener("click", async () => {
    try {
      const out = await post<{ redirect?: string }>("/auth/logout", {});
      location.assign(out.redirect || "/");
    } catch {
      location.assign("/");
    }
  });
}

function main() {
  // Looping cover videos stay still for visitors who prefer reduced motion.
  if (matchMedia("(prefers-reduced-motion: reduce)").matches) {
    document.querySelectorAll<HTMLVideoElement>("video[autoplay]").forEach((v) => {
      v.autoplay = false;
      v.pause();
    });
  }
  document.querySelectorAll<HTMLElement>("[data-signout]").forEach(bindSignOut);
  document.querySelectorAll<HTMLElement>("[data-checkout]").forEach((box) => {
    bindCheckout(box).catch(() => {
      /* links keep pointing to sign-in */
    });
  });
  document.querySelectorAll<HTMLElement>("[data-actions]").forEach((box) => {
    renderActions(box).catch(() => {
      /* keep the server-rendered fallback */
    });
  });
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", main);
else main();
