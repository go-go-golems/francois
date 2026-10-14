// public.ts adds the session-dependent actions to server-rendered storefront
// pages: sign out, run an artifact, try a demo. Pages are complete without
// it; it only replaces placeholders. No framework, to stay small.

type Session = {
  authenticated: boolean;
  loginAvailable: boolean;
  csrfToken?: string;
  user?: { role: string; disabled: boolean };
  access?: { allowed: boolean };
};

type Failure = { error?: { code?: string; message?: string } };

let session: Promise<Session> | undefined;
const getSession = () => (session ??= fetch("/api/session", { credentials: "same-origin" }).then((r) => r.json() as Promise<Session>));

async function post<T>(path: string, body: unknown): Promise<T> {
  const s = await getSession();
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (s.csrfToken) headers["X-CSRF-Token"] = s.csrfToken;
  const r = await fetch(path, { method: "POST", credentials: "same-origin", headers, body: JSON.stringify(body) });
  const out = (await r.json().catch(() => ({}))) as T & Failure;
  if (!r.ok) throw new Error(out.error?.message || "Request failed");
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
  box.append(el("p", { class: "action-error notice bad", role: "alert" }, message));
}

async function renderActions(box: HTMLElement) {
  const s = await getSession();
  const { release, title, demo } = box.dataset;
  const meta = box.querySelector(".muted");
  box.replaceChildren();
  if (release) {
    if (!s.authenticated) {
      box.append(el("a", { class: "btn btn-ink", href: loginHref() }, "Sign in to run"));
    } else if (s.access?.allowed) {
      const run = el("button", { class: "btn btn-ink", type: "button" }, "Run in a new instance");
      run.addEventListener("click", async () => {
        run.setAttribute("disabled", "");
        try {
          const inst = await post<{ id: string }>("/api/instances", { releaseId: release, title });
          location.assign("/instances/" + inst.id);
        } catch (e) {
          run.removeAttribute("disabled");
          fail(box, (e as Error).message);
        }
      });
      box.append(run);
    } else {
      box.append(el("a", { class: "btn btn-ink", href: "/account?subscribe=1" }, "Subscribe to run"));
    }
  }
  if (demo === "true") {
    const btn = el("button", { class: "btn btn-soft", type: "button" }, "Try the demo");
    btn.addEventListener("click", async () => {
      btn.setAttribute("disabled", "");
      try {
        const inst = await post<{ id: string }>("/api/demos", { artifactId: box.dataset.artifact });
        location.assign("/instances/" + inst.id);
      } catch (e) {
        btn.removeAttribute("disabled");
        fail(box, (e as Error).message);
      }
    });
    box.append(btn);
  }
  if (meta) box.append(meta);
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
  document.querySelectorAll<HTMLElement>("[data-signout]").forEach(bindSignOut);
  document.querySelectorAll<HTMLElement>("[data-actions]").forEach((box) => {
    renderActions(box).catch(() => {
      /* keep the server-rendered fallback */
    });
  });
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", main);
else main();
