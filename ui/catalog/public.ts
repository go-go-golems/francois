import { bindInstanceLaunch } from "./instance-launch";
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
const getSession = () =>
  (session ??= fetch("/api/session", { credentials: "same-origin" }).then(
    (r) => r.json() as Promise<Session>,
  ));

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
  if (!r.ok)
    throw new RequestError(
      out.error?.message || "Request failed",
      out.error?.code || "",
    );
  return out;
}

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Record<string, string>,
  text?: string,
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  if (text) e.textContent = text;
  return e;
}

const loginHref = () =>
  "/auth/login?return=" +
  encodeURIComponent(location.pathname + location.search);

function fail(box: HTMLElement, message: string) {
  box.querySelector(".action-error")?.remove();
  const notice = serviceNotice(message);
  notice.classList.add("action-error");
  box.append(notice);
}

async function renderActions(box: HTMLElement) {
  const s = await getSession();
  const { release, title, demo } = box.dataset;
  const required = Number(box.dataset.rank ?? "1");
  const plan = s.plan;
  const canRun =
    !!plan &&
    (plan.source === "staff" ||
      (plan.source !== "guest" &&
        plan.source !== "disabled" &&
        plan.rank >= required &&
        plan.instances > 0));
  const runner = box.closest<HTMLElement>("[data-runner]");
  box.replaceChildren();
  if (!release) return;
  if (!canRun && demo !== "true") {
    box.append(
      el("a", { class: "site-button", href: loginHref() }, "Sign in to run"),
    );
    return;
  }
  const start = el(
    "button",
    { class: "site-button", type: "button" },
    "Start experiment →",
  );
  box.append(start);
  let instance:
    | { id: string; origin: string; launchBlocked?: string }
    | undefined;
  let unbind: (() => void) | undefined;
  start.addEventListener("click", async () => {
    start.disabled = true;
    start.textContent = "Starting…";
    box.querySelector(".action-error")?.remove();
    try {
      if (!instance) {
        const created = canRun
          ? await post<{ id: string }>("/api/instances/open", {
              releaseId: release,
              title,
            })
          : await post<{ id: string }>("/api/demos", {
              artifactId: box.dataset.artifact,
            });
        // A new guest visit sets a cookie. Discard the pre-guest CSRF snapshot.
        session = undefined;
        await getSession();
        const response = await fetch("/api/instances/" + created.id);
        const out = await response.json();
        if (!response.ok)
          throw new RequestError(
            out.error?.message || "Could not open the experiment",
            out.error?.code || "",
          );
        instance = out;
      }
      if (!instance) throw new Error("Could not open the experiment");
      if (instance.launchBlocked) throw new Error(instance.launchBlocked);
      if (!runner) {
        location.assign("/instances/" + instance.id);
        return;
      }
      const surface = runner.querySelector<HTMLElement>(
        "[data-runner-surface]",
      )!;
      const state = runner.querySelector<HTMLElement>("[data-runner-state]")!;
      const frame = el("iframe", {
        title: title || "Experiment",
        sandbox:
          "allow-scripts allow-same-origin allow-forms allow-popups allow-modals allow-downloads",
        allow: "clipboard-write; fullscreen",
        allowfullscreen: "",
      });
      const current = instance;
      unbind?.();
      unbind = bindInstanceLaunch(
        frame,
        current.origin,
        (challenge) =>
          post("/api/instances/" + current.id + "/launch", { challenge }),
        (message) => {
          state.textContent = message ? "Unavailable" : "Live";
          runner.classList.toggle("is-live", !message);
          if (message) fail(runner, message);
          else runner.querySelector(".action-error")?.remove();
        },
      );
      frame.src = current.origin + "/launch";
      surface.replaceChildren(frame);
      state.textContent = "Starting…";
      runner.querySelector<HTMLAnchorElement>("[data-runner-link]")!.href =
        "/instances/" + current.id;
      runner
        .querySelectorAll<HTMLElement>("[data-runner-control]")
        .forEach((e) => (e.hidden = false));
      box.hidden = true;
      runner.querySelector<HTMLButtonElement>("[data-runner-reload]")!.onclick =
        () => {
          runner.querySelector(".action-error")?.remove();
          state.textContent = "Starting…";
          runner.classList.remove("is-live");
          frame.src = current.origin + "/launch";
        };
      runner.querySelector<HTMLButtonElement>(
        "[data-runner-fullscreen]",
      )!.onclick = async () => {
        if (surface.requestFullscreen) {
          try {
            await surface.requestFullscreen();
          } catch {
            fail(runner, "Use Open full screen to open the experiment.");
          }
        } else location.assign("/instances/" + current.id);
      };
    } catch (e) {
      start.disabled = false;
      start.textContent = "Try again →";
      fail(box, (e as Error).message);
    }
  });
  addEventListener("pagehide", (event) => {
    if (!event.persisted) unbind?.();
  });
}

function bindArchive() {
  document
    .querySelectorAll<HTMLImageElement>(".archive-thumb img")
    .forEach((image) => {
      const fallback = () => {
        image.hidden = true;
      };
      image.addEventListener("error", fallback);
      if (image.complete && image.naturalWidth === 0) fallback();
    });
  document
    .querySelector<HTMLSelectElement>("[data-archive-sort]")
    ?.addEventListener("change", (event) => {
      (event.target as HTMLSelectElement).form?.requestSubmit();
    });
  const preview = document.querySelector<HTMLElement>("[data-archive-preview]");
  if (!preview) return;
  document
    .querySelectorAll<HTMLElement>("[data-archive-row]")
    .forEach((row) => {
      const show = () => {
        if (!matchMedia("(min-width: 901px) and (hover: hover)").matches)
          return;
        const media = row.querySelector(
          "img:not([hidden]),video:not([hidden])",
        );
        if (!media) return;
        const clone = media.cloneNode(true) as HTMLElement;
        clone.setAttribute("aria-hidden", "true");
        preview.replaceChildren(clone);
        preview.hidden = false;
        const rect = row.getBoundingClientRect();
        preview.style.left =
          Math.min(
            innerWidth - preview.offsetWidth - 20,
            rect.left + rect.width * 0.46,
          ) + "px";
        preview.style.top =
          Math.max(
            12,
            Math.min(innerHeight - preview.offsetHeight - 12, rect.top + 24),
          ) + "px";
      };
      const hide = () => {
        preview.hidden = true;
      };
      row.addEventListener("mouseenter", show);
      row.addEventListener("focus", show);
      row.addEventListener("mouseleave", hide);
      row.addEventListener("blur", hide);
    });
  addEventListener(
    "scroll",
    () => {
      preview.hidden = true;
    },
    { passive: true },
  );
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
  bindArchive();
  // Looping cover videos stay still for visitors who prefer reduced motion.
  if (matchMedia("(prefers-reduced-motion: reduce)").matches) {
    document
      .querySelectorAll<HTMLVideoElement>("video[autoplay]")
      .forEach((v) => {
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

if (document.readyState === "loading")
  document.addEventListener("DOMContentLoaded", main);
else main();
