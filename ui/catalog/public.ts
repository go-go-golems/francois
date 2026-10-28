import { bindInstanceLaunch } from "./instance-launch";
import {
  autoStart,
  decide,
  inNewTab,
  type Decision,
  type LaunchData,
  type Plan,
} from "./launch-decision";
import {
  ENTERPRISE_DEFAULT,
  isEnterpriseAmount,
  shift,
  type Shift,
} from "./enterprise-amount";
import { serviceNotice } from "./service-notice";
// public.ts adds the session-dependent actions to server-rendered storefront
// pages: sign out, run an artifact (in the page, in the landing page viewer
// or full screen in a new tab), try a demo, and the landing page's live
// search. Pages are complete without it; it only replaces placeholders and
// intercepts links. No framework, to stay small.

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
  "/signin?return=" + encodeURIComponent(location.pathname + location.search);

function fail(box: HTMLElement, message: string) {
  box.querySelector(".action-error")?.remove();
  const notice = serviceNotice(message);
  notice.classList.add("action-error");
  box.append(notice);
}

// Labels come from the page (site copy) so editors can change them.
const label = (box: HTMLElement, key: string, fallback: string) =>
  box.dataset[key] || fallback;

type Opened = { id: string; origin: string; launchBlocked?: string };

// openInstance creates (or resumes) the visitor's instance, or a demo for a
// visitor without a plan, and returns where to frame it.
async function openInstance(
  data: DOMStringMap,
  decision: Decision,
): Promise<Opened> {
  const created =
    decision.kind === "start" && decision.demo
      ? await post<{ id: string }>("/api/demos", { artifactId: data.artifact })
      : await post<{ id: string }>("/api/instances/open", {
          releaseId: data.release,
          title: data.title,
        });
  // A new guest visit sets a cookie. Discard the pre-guest CSRF snapshot.
  session = undefined;
  await getSession();
  const response = await fetch("/api/instances/" + created.id);
  const out = await response.json();
  if (!response.ok)
    throw new RequestError(
      out.error?.message || "Could not open the artifact",
      out.error?.code || "",
    );
  if (out.launchBlocked) throw new Error(out.launchBlocked);
  return out as Opened;
}

// mountFrame frames an opened instance in surface and runs the launch
// handshake; onState receives "" when the app started, else the reason.
function mountFrame(
  surface: HTMLElement,
  instance: Opened,
  title: string,
  onState: (message: string) => void,
) {
  const frame = el("iframe", {
    title: title || "Artifact",
    sandbox:
      "allow-scripts allow-same-origin allow-forms allow-popups allow-modals allow-downloads",
    allow: "clipboard-write; fullscreen",
  });
  const unbind = bindInstanceLaunch(
    frame,
    instance.origin,
    (challenge) =>
      post("/api/instances/" + instance.id + "/launch", { challenge }),
    onState,
  );
  frame.src = instance.origin + "/launch";
  surface.replaceChildren(frame);
  return {
    unbind,
    reload: () => {
      frame.src = instance.origin + "/launch";
    },
  };
}

// openInNewTab opens the window synchronously (inside the click, so popup
// blockers allow it) on the page that starts the instance, so the tab shows
// the application starting rather than staying blank. That page starts only
// for a tab the storefront opened; when the popup is blocked, this tab
// creates the instance and then goes to it.
async function openInNewTab(data: DOMStringMap, decision: Decision) {
  const q = new URLSearchParams({
    artifact: data.artifact ?? "",
    release: data.release ?? "",
    title: data.title ?? "",
  });
  if (decision.kind === "start" && decision.demo) q.set("demo", "1");
  if (window.open("/instances/new?" + q, "_blank")) return;
  const instance = await openInstance(data, decision);
  location.assign("/instances/" + instance.id);
}

async function renderActions(box: HTMLElement) {
  const s = await getSession();
  const data = box.dataset as LaunchData & DOMStringMap;
  const decision = decide(s.plan, data);
  const runner = box.closest<HTMLElement>("[data-runner]");
  box.replaceChildren();
  if (!data.release) return;
  if (decision.kind === "subscribe") {
    box.append(
      el("p", { class: "runner-early" }, label(box, "labelEarly", "")),
      el(
        "a",
        { class: "site-button is-primary", href: "/pricing" },
        label(box, "labelSubscribe", "Subscribe →"),
      ),
    );
    return;
  }
  if (decision.kind === "signin") {
    box.append(
      el(
        "a",
        { class: "site-button", href: loginHref() },
        label(box, "labelSignin", "Sign in to run"),
      ),
    );
    return;
  }
  const text = decision.demo
    ? label(box, "labelDemo", "Try it →")
    : label(box, "labelStart", "Run artifact →");
  const start = el(
    "button",
    { class: "site-button is-primary", type: "button" },
    text,
  );
  box.append(start);
  let instance: Opened | undefined;
  let mounted: ReturnType<typeof mountFrame> | undefined;
  const run = async () => {
    start.disabled = true;
    start.textContent = "Starting…";
    box.querySelector(".action-error")?.remove();
    try {
      if (inNewTab(data) || !runner) {
        await openInNewTab(data, decision);
        start.disabled = false;
        start.textContent = text;
        return;
      }
      instance ??= await openInstance(data, decision);
      const current = instance;
      const surface = runner.querySelector<HTMLElement>(
        "[data-runner-surface]",
      )!;
      const state = runner.querySelector<HTMLElement>("[data-runner-state]")!;
      mounted?.unbind();
      mounted = mountFrame(surface, current, data.title || "", (message) => {
        state.textContent = message ? "Unavailable" : "Live";
        runner.classList.toggle("is-live", !message);
        if (message) fail(runner, message);
        else runner.querySelector(".action-error")?.remove();
      });
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
          mounted?.reload();
        };
      runner.querySelector<HTMLButtonElement>(
        "[data-runner-fullscreen]",
      )!.onclick = async () => {
        if (surface.requestFullscreen) {
          try {
            await surface.requestFullscreen();
          } catch {
            fail(runner, "Use Open full screen to open the artifact.");
          }
        } else location.assign("/instances/" + current.id);
      };
    } catch (e) {
      start.disabled = false;
      start.textContent = "Try again →";
      fail(box, (e as Error).message);
    }
  };
  start.addEventListener("click", run);
  addEventListener("pagehide", (event) => {
    if (!event.persisted) mounted?.unbind();
  });
  if (runner && autoStart(data, decision)) void run();
}

// Viewer is the landing page's full-viewport window for inline artifacts,
// after html.moldandyeast.com: title, Details, Close · Esc, device sizes.
class Viewer {
  private unbind?: () => void;
  private reload?: () => void;
  private opened?: string;

  constructor(private dialog: HTMLDialogElement) {
    dialog.addEventListener("cancel", (event) => {
      event.preventDefault();
      this.close(true);
    });
    addEventListener("popstate", () => {
      if (this.opened && !new URLSearchParams(location.search).get("open"))
        this.close(false);
    });
  }

  async open(data: DOMStringMap, decision: Decision, push = true) {
    this.close(false);
    const id = data.artifact!;
    this.opened = id;
    if (push) {
      const url = new URL(location.href);
      url.searchParams.set("open", id);
      history.pushState({ open: id }, "", url);
    }
    const surface = el("div", { class: "v-device" });
    const state = el("span", { class: "v-state" }, "Starting…");
    const close = el(
      "button",
      { class: "tlink", type: "button" },
      "Close · Esc",
    );
    close.onclick = () => this.close(true);
    const sizes = el("div", {
      class: "v-sizes",
      role: "group",
      "aria-label": "Screen size",
    });
    for (const [name, width] of [
      ["Phone", "390px"],
      ["Tablet", "834px"],
      ["Desktop", "100%"],
    ]) {
      const b = el(
        "button",
        { type: "button", "aria-pressed": String(name === "Desktop") },
        name,
      );
      b.onclick = () => {
        surface.style.maxWidth = width;
        sizes
          .querySelectorAll("button")
          .forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
      };
      sizes.append(b);
    }
    const reload = el("button", { class: "tlink", type: "button" }, "Reload");
    reload.onclick = () => this.reload?.();
    const full = el(
      "a",
      { class: "u", target: "_blank", rel: "noopener" },
      "Open in new tab ↗",
    );
    const title = el("div", { class: "v-title" });
    title.append(
      el("strong", {}, data.title || ""),
      el("span", { class: "v-desc" }, data.description || ""),
    );
    const actions = el("div", { class: "v-actions" });
    actions.append(
      el("a", { class: "u", href: "/apps/" + id }, "Details"),
      close,
    );
    const top = el("div", { class: "v-top" });
    top.append(title, actions);
    const table = el("div", { class: "v-table" });
    table.append(surface);
    const tools = el("div", { class: "v-actions" });
    tools.append(state, reload, full);
    const bottom = el("div", { class: "v-bottom" });
    bottom.append(sizes, tools);
    this.dialog.replaceChildren(top, table, bottom);
    this.dialog.setAttribute("aria-label", data.title || "Artifact");
    document.body.classList.add("has-window");
    this.dialog.showModal();
    close.focus();
    try {
      const instance = await openInstance(data, decision);
      if (this.opened !== id) return;
      full.href = "/instances/" + instance.id;
      const mounted = mountFrame(
        surface,
        instance,
        data.title || "",
        (message) => {
          state.textContent = message || "Live";
        },
      );
      this.unbind = mounted.unbind;
      this.reload = mounted.reload;
    } catch (e) {
      if (this.opened !== id) return;
      const box = el("div", { class: "v-blocked" });
      box.append(
        el("p", {}, (e as Error).message),
        el("a", { class: "u", href: "/apps/" + id }, "Go to the artifact page"),
      );
      surface.replaceChildren(box);
      state.textContent = "Unavailable";
    }
  }

  close(back: boolean) {
    if (!this.opened) return;
    this.opened = undefined;
    this.unbind?.();
    this.unbind = this.reload = undefined;
    this.dialog.close();
    this.dialog.replaceChildren();
    document.body.classList.remove("has-window");
    if (back && new URLSearchParams(location.search).get("open")) {
      const url = new URL(location.href);
      url.searchParams.delete("open");
      history.pushState({}, "", url);
    }
  }
}

function bindBrowser() {
  document
    .querySelector<HTMLSelectElement>("[data-archive-sort]")
    ?.addEventListener("change", (event) => {
      (event.target as HTMLSelectElement).form?.requestSubmit();
    });
  const list = document.querySelector<HTMLElement>("[data-browser-list]");
  if (!list) return;

  // Live filter: hide rows and groups that do not match while typing; Enter
  // submits the form and the server applies the same rule.
  const query = document.querySelector<HTMLInputElement>(
    "[data-browser-query]",
  );
  const count = document.querySelector<HTMLElement>("[data-browser-count]");
  const empty = list.querySelector<HTMLElement>("[data-browser-empty]");
  const rows = [...list.querySelectorAll<HTMLElement>("[data-browser-row]")];
  const haystacks = rows.map((r) => (r.dataset.search || "").toLowerCase());
  let timer: number | undefined;
  query?.addEventListener("input", () => {
    clearTimeout(timer);
    timer = window.setTimeout(() => {
      const q = query.value.trim().toLowerCase();
      let shown = 0;
      rows.forEach((r, i) => {
        r.hidden = !!q && !haystacks[i].includes(q);
        if (!r.hidden) shown++;
      });
      list
        .querySelectorAll<HTMLElement>("[data-browser-group]")
        .forEach((g) => {
          const n = g.querySelectorAll(
            "[data-browser-row]:not([hidden])",
          ).length;
          g.hidden = n === 0;
          const label = g.querySelector<HTMLElement>(
            "[data-browser-group-count]",
          );
          if (label)
            label.textContent = `${n} ${n === 1 ? "artifact" : "artifacts"}`;
        });
      if (count)
        count.textContent = `${shown} ${shown === 1 ? "artifact" : "artifacts"}`;
      if (empty) empty.hidden = shown > 0;
      const url = new URL(location.href);
      if (q) url.searchParams.set("q", query.value.trim());
      else url.searchParams.delete("q");
      history.replaceState(history.state, "", url);
    }, 80);
  });

  // Open: inline artifacts start in the viewer, tab artifacts open full
  // screen in a new tab. Everyone else follows the link to the app page,
  // which explains how to get access. The session is fetched up front so
  // the decision is made synchronously inside the click (popup blockers).
  const dialog = document.querySelector<HTMLDialogElement>("[data-viewer]");
  const viewer = dialog ? new Viewer(dialog) : undefined;
  let plan: Plan | undefined;
  let ready = false;
  getSession()
    .then((s) => {
      plan = s.plan;
      ready = true;
      const open = new URLSearchParams(location.search).get("open");
      const link =
        open &&
        list.querySelector<HTMLAnchorElement>(
          `[data-open][data-artifact="${CSS.escape(open)}"]`,
        );
      // On page load, only inline artifacts for visitors who may run them.
      if (link && viewer) {
        const d = decide(plan, link.dataset);
        if (autoStart(link.dataset, d))
          void viewer.open(link.dataset, d, false);
      }
    })
    .catch(() => {});
  list.addEventListener("click", (event) => {
    const link = (event.target as Element).closest<HTMLAnchorElement>(
      "[data-open]",
    );
    if (
      !link ||
      !ready ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.button !== 0
    )
      return;
    const d = decide(plan, link.dataset);
    if (d.kind !== "start") return;
    event.preventDefault();
    if (inNewTab(link.dataset))
      openInNewTab(link.dataset, d).catch((e) =>
        fail(
          link.closest<HTMLElement>(".ri-foot") || list,
          (e as Error).message,
        ),
      );
    else if (viewer) void viewer.open(link.dataset, d);
  });
}

// startSupport opens Stripe Checkout for monthly support. Visitors without
// a verified account sign in first and come back to the same choice; an
// open subscription is managed in the Stripe portal instead.
async function startSupport(
  form: HTMLFormElement,
  submit: HTMLButtonElement,
  body: { kind?: string; amountCents: number },
  returnTo: string,
) {
  if (submit.disabled || !form.reportValidity()) return;
  submit.disabled = true;
  try {
    const session = await getSession();
    if (!session.authenticated || session.user?.role === "guest") {
      location.assign("/signin?return=" + encodeURIComponent(returnTo));
      return;
    }
    const result = await post<{ url: string }>("/api/billing/support", body);
    location.assign(result.url);
  } catch (err) {
    if (err instanceof RequestError && err.code === "subscription_open") {
      try {
        const result = await post<{ url: string }>("/api/billing/portal", {});
        location.assign(result.url);
        return;
      } catch (portalError) {
        err = portalError;
      }
    }
    fail(form, (err as Error).message);
    submit.disabled = false;
  }
}

const enterpriseLink = () =>
  new URLSearchParams(location.search).get("tier") === "enterprise";

function bindSupport(form: HTMLFormElement) {
  const input = form.querySelector<HTMLInputElement>("[name=amount]")!;
  const submit = form.querySelector<HTMLButtonElement>("[type=submit]")!;
  const resize = () => {
    input.style.width = Math.max(2, input.value.length) + "ch";
  };
  const initial = Number(new URLSearchParams(location.search).get("amount"));
  if (
    !enterpriseLink() &&
    Number.isInteger(initial) &&
    initial >= 1 &&
    initial <= 9999
  )
    input.value = String(initial);
  resize();
  input.addEventListener("input", resize);
  form
    .querySelectorAll<HTMLButtonElement>("[data-amount-step]")
    .forEach((button) => {
      button.addEventListener("click", () => {
        const current = Number(input.value);
        input.value = String(
          Math.min(
            9999,
            Math.max(
              1,
              (Number.isFinite(current) ? Math.round(current) : 42) +
                Number(button.dataset.amountStep),
            ),
          ),
        );
        resize();
      });
    });
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    void startSupport(
      form,
      submit,
      { amountCents: Number(input.value) * 100 },
      "/pricing?amount=" + input.value,
    );
  });
}

// bindEnterprise drives the enterprise amount: <<1 halves and >>1 doubles,
// between 8 and 4096 USD; the buttons are disabled at either end.
function bindEnterprise(form: HTMLFormElement) {
  const output = form.querySelector<HTMLOutputElement>("output[name=amount]")!;
  const submit = form.querySelector<HTMLButtonElement>("[type=submit]")!;
  const buttons = [...form.querySelectorAll<HTMLButtonElement>("[data-shift]")];
  const initial = Number(new URLSearchParams(location.search).get("amount"));
  let amount =
    enterpriseLink() && isEnterpriseAmount(initial)
      ? initial
      : ENTERPRISE_DEFAULT;
  const render = () => {
    output.value = String(amount);
    for (const b of buttons)
      b.disabled = shift(amount, b.dataset.shift as Shift) === amount;
  };
  for (const b of buttons)
    b.addEventListener("click", () => {
      amount = shift(amount, b.dataset.shift as Shift);
      render();
    });
  render();
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    void startSupport(
      form,
      submit,
      { kind: "enterprise", amountCents: amount * 100 },
      `/pricing?tier=enterprise&amount=${amount}`,
    );
  });
}

// bindSupportTiers turns the two support panels into tabs. Without
// JavaScript both panels show, one under the other.
function bindSupportTiers(box: HTMLElement) {
  const list = box.querySelector<HTMLElement>("[role=tablist]")!;
  const tabs = [...list.querySelectorAll<HTMLButtonElement>("[role=tab]")];
  const select = (tab: HTMLButtonElement, focus: boolean) => {
    for (const t of tabs) {
      const on = t === tab;
      t.setAttribute("aria-selected", String(on));
      t.tabIndex = on ? 0 : -1;
      document.getElementById(t.getAttribute("aria-controls")!)!.hidden = !on;
    }
    if (focus) tab.focus();
  };
  tabs.forEach((tab, i) => {
    tab.addEventListener("click", () => {
      select(tab, false);
      const url = new URL(location.href);
      url.searchParams.delete("amount");
      if (tab.dataset.tier) url.searchParams.set("tier", tab.dataset.tier);
      else url.searchParams.delete("tier");
      history.replaceState(history.state, "", url);
    });
    tab.addEventListener("keydown", (event) => {
      const step =
        event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
      if (!step) return;
      event.preventDefault();
      tabs[(i + step + tabs.length) % tabs.length].click();
      tabs[(i + step + tabs.length) % tabs.length].focus();
    });
  });
  list.hidden = false;
  select(tabs[enterpriseLink() ? 1 : 0], false);
}

function bindYearSupport(form: HTMLFormElement) {
  const button = form.querySelector<HTMLButtonElement>("button")!;
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (button.disabled) return;
    button.disabled = true;
    try {
      const session = await getSession();
      if (!session.authenticated || session.user?.role === "guest") {
        location.assign(
          "/signin?return=" + encodeURIComponent("/pricing?fund=year"),
        );
        return;
      }
      const result = await post<{ url: string }>("/api/billing/support", {
        kind: "year",
      });
      location.assign(result.url);
    } catch (err) {
      fail(form, (err as Error).message);
      button.disabled = false;
    }
  });
}

// bindLightbox opens a gallery's screenshots full size in a modal: arrows
// or the buttons step through them, Esc, Close or a click outside the image
// closes it. Without JavaScript the link opens the image itself.
function bindLightbox(gallery: HTMLElement) {
  const links = [
    ...gallery.querySelectorAll<HTMLAnchorElement>("a[data-zoom]"),
  ];
  if (!links.length) return;
  const dialog = el("dialog", {
    class: "lightbox",
    "aria-label": "Screenshot",
  });
  const img = el("img", { alt: "" });
  const caption = el("p", { class: "lightbox-caption" });
  const count = el("span", { class: "lightbox-count" });
  const prev = el(
    "button",
    { type: "button", class: "tlink", "aria-label": "Previous screenshot" },
    "← Previous",
  );
  const next = el(
    "button",
    { type: "button", class: "tlink", "aria-label": "Next screenshot" },
    "Next →",
  );
  const close = el("button", { type: "button", class: "tlink" }, "Close · Esc");
  const bar = el("div", { class: "lightbox-bar" });
  const nav = el("div", { class: "lightbox-nav" });
  if (links.length > 1) nav.append(prev, count, next);
  bar.append(caption, nav, close);
  dialog.append(img, bar);
  document.body.append(dialog);
  let at = 0;
  const show = (i: number) => {
    at = (i + links.length) % links.length;
    const thumb = links[at].querySelector("img");
    img.src = links[at].href;
    img.alt = thumb?.alt || "";
    caption.textContent = thumb?.alt || "";
    count.textContent = `${at + 1} / ${links.length}`;
  };
  links.forEach((link, i) =>
    link.addEventListener("click", (event) => {
      if (
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.button !== 0
      )
        return;
      event.preventDefault();
      show(i);
      dialog.showModal();
    }),
  );
  prev.onclick = () => show(at - 1);
  next.onclick = () => show(at + 1);
  close.onclick = () => dialog.close();
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) dialog.close();
  });
  dialog.addEventListener("keydown", (event) => {
    if (event.key === "ArrowLeft") show(at - 1);
    else if (event.key === "ArrowRight") show(at + 1);
  });
  dialog.addEventListener("close", () => links[at].focus());
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
  bindBrowser();
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
  document.querySelectorAll<HTMLElement>(".gallery").forEach(bindLightbox);
  document
    .querySelectorAll<HTMLFormElement>("[data-support-monthly]")
    .forEach(bindSupport);
  document
    .querySelectorAll<HTMLFormElement>("[data-support-enterprise]")
    .forEach(bindEnterprise);
  document
    .querySelectorAll<HTMLElement>("[data-support-tiers]")
    .forEach(bindSupportTiers);
  document
    .querySelectorAll<HTMLFormElement>("[data-support-year]")
    .forEach(bindYearSupport);
  document.querySelectorAll<HTMLElement>("[data-actions]").forEach((box) => {
    renderActions(box).catch(() => {
      /* keep the server-rendered fallback */
    });
  });
}

if (document.readyState === "loading")
  document.addEventListener("DOMContentLoaded", main);
else main();
