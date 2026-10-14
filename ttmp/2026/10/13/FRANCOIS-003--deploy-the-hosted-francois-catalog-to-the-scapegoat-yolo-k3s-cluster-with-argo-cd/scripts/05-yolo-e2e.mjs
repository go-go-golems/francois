// Drives the real yolo deployment: ZITADEL Login V2 (invite-only Francois
// organization), Francois portal, Stripe Checkout (sandbox), instance launch
// on the isolated app origin and sign-out through ZITADEL. Never prints
// credentials. Each step uses a fresh browser context unless it reuses the
// stored state of a previous step.
import { createRequire } from "node:module";
import { existsSync } from "node:fs";
const require = createRequire(process.env.NODE_PATH + "/");
const { chromium } = require("playwright-core");

const BASE = "https://francois.yolo.scapegoat.dev";
const creds = JSON.parse(process.env.E2E_CREDS);
const out = process.env.E2E_OUT;
const step = process.argv[2] ?? "admin";
const log = (...a) => console.log(`[${step}]`, ...a);

async function login(page, user, pass) {
  await page.goto(BASE + "/auth/login?return=/account");
  await page.waitForURL(/zitadel\.yolo\.scapegoat\.dev/);
  const name = page.getByRole("textbox", { name: "Username or email" });
  if (await name.isVisible().catch(() => false)) {
    await name.fill(user);
    await page.getByRole("button", { name: "Continue" }).click();
  }
  const pw = page.locator('input[type="password"]');
  await pw.waitFor({ timeout: 20000 });
  await pw.fill(pass);
  await page.getByRole("button", { name: "Continue" }).click();
  // ZITADEL may offer second-factor setup (mfa_init_skip_lifetime); skip it.
  for (let i = 0; i < 3 && !page.url().startsWith(BASE); i++) {
    const skip = page.getByRole("button", { name: /skip/i });
    await Promise.race([page.waitForURL((u) => u.href.startsWith(BASE), { timeout: 15000 }).catch(() => {}), skip.waitFor({ timeout: 15000 }).catch(() => {})]);
    if (!page.url().startsWith(BASE) && (await skip.isVisible().catch(() => false))) await skip.click();
  }
  await page.waitForURL((u) => u.href.startsWith(BASE), { timeout: 30000 });
}

async function session(page) {
  return page.evaluate(async () => (await fetch("/api/session")).json());
}

const browser = await chromium.launch({ executablePath: process.env.CHROME, headless: true });
const statePath = (who) => `${out}/state-${who}.json`;
try {
  if (step === "admin") {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await login(page, creds.admin_username, creds.admin_password);
    const s = await session(page);
    log("session", JSON.stringify({ authenticated: s.authenticated, role: s.user?.role, email: s.user?.email, access: s.access?.reason }));
    await page.goto(BASE + "/admin");
    await page.waitForTimeout(1500);
    await page.screenshot({ path: `${out}/01-admin.png`, fullPage: true });
    log("admin page title:", (await page.locator("h1").first().textContent())?.trim());
  } else if (step === "alice-subscribe") {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await login(page, creds.alice_username, creds.alice_password);
    let s = await session(page);
    log("before", JSON.stringify({ role: s.user?.role, allowed: s.access?.allowed, reason: s.access?.reason }));
    // FRANCOIS-004: subscribing starts from the pricing page (first paid plan,
    // monthly price).
    await page.goto(BASE + "/pricing");
    await page.locator("[data-checkout] a[data-interval=month]").first().click();
    await page.waitForURL(/checkout\.stripe\.com/, { timeout: 30000 });
    log("on Stripe Checkout");
    await page.waitForTimeout(3000);
    await page.screenshot({ path: `${out}/02-checkout.png`, fullPage: true });
    // Newer Checkout pages list payment methods as an accordion; pick Card.
    // The accordion's click target is a transparent overlay above the label;
    // a forced click at the label's position lands on it.
    await page.getByText("Card", { exact: true }).first().click({ force: true, timeout: 10000 });
    const card = page.locator("#cardNumber");
    if (!(await card.waitFor({ timeout: 15000 }).then(() => true).catch(() => false))) {
      await page.screenshot({ path: `${out}/02b-checkout-card.png`, fullPage: true });
      for (const f of page.frames()) {
        const ins = await f.evaluate(() => [...document.querySelectorAll("input,select")].map((e) => `${e.tagName.toLowerCase()}#${e.id}[name=${e.name}][ac=${e.autocomplete}]`)).catch(() => []);
        if (ins.length) log("frame", f.url().slice(0, 80), JSON.stringify(ins));
      }
      throw new Error("card fields not found");
    }
    await card.fill("4242424242424242");
    await page.locator("#cardExpiry").fill("12 / 34");
    await page.locator("#cardCvc").fill("123");
    const nameField = page.locator("#billingName");
    if (await nameField.isVisible().catch(() => false)) await nameField.fill("E2E Alice");
    const zip = page.locator("#billingPostalCode");
    if (await zip.isVisible().catch(() => false)) await zip.fill("10001");
    // Link opt-in is pre-checked and would demand a phone number.
    const link = page.getByRole("checkbox", { name: /save my information/i });
    if (await link.count()) await link.first().uncheck({ force: true }).catch(() => {});
    await page.locator('button[type="submit"]').first().click();
    if (!(await page.waitForURL((u) => u.href.startsWith(BASE), { timeout: 60000 }).then(() => true).catch(() => false))) {
      await page.screenshot({ path: `${out}/02c-checkout-submitted.png`, fullPage: true });
      throw new Error("Checkout did not return to the portal");
    }
    log("returned to", new URL(page.url()).pathname + new URL(page.url()).search.replace(/session_id=[^&]+/, "session_id=…"));
    for (let i = 0; i < 30; i++) {
      s = await session(page);
      if (s.access?.allowed) break;
      await page.waitForTimeout(2000);
    }
    log("after", JSON.stringify({ allowed: s.access?.allowed, reason: s.access?.reason }));
    await page.screenshot({ path: `${out}/03-account-after.png`, fullPage: true });
    await ctx.storageState({ path: statePath("alice") });
  } else if (step === "alice-run") {
    if (!existsSync(statePath("alice"))) throw new Error("run alice-subscribe first");
    const ctx = await browser.newContext({ storageState: statePath("alice") });
    const page = await ctx.newPage();
    await page.goto(BASE + "/apps/polis");
    await page.getByRole("button", { name: /run in a new private instance/i }).click();
    await page.waitForURL(/\/instances\//, { timeout: 30000 });
    const frameEl = page.locator("iframe").first();
    await frameEl.waitFor({ timeout: 30000 });
    let appFrame;
    for (let i = 0; i < 30 && !appFrame; i++) {
      appFrame = page.frames().find((f) => /^https:\/\/i-[^.]+\.apps\.francois\.yolo\.scapegoat\.dev\//.test(f.url()));
      if (!appFrame) await page.waitForTimeout(1000);
    }
    if (!appFrame) throw new Error("no instance frame on the app origin");
    const origin = new URL(appFrame.url()).origin;
    log("instance origin", origin.replace(/i-[^.]+/, "i-…"));
    await page.waitForTimeout(4000);
    const info = await appFrame.evaluate(() => ({ title: document.title, bytes: document.documentElement.outerHTML.length, cookieVisible: document.cookie.includes("francois") }));
    log("instance document", JSON.stringify(info));
    // The portal's session cookie must not be readable or sent cross-origin.
    const probe = await appFrame.evaluate(async (base) => {
      try { const r = await fetch(base + "/api/session", { credentials: "include" }); return "status " + r.status; } catch (e) { return "blocked: " + e.name; }
    }, BASE);
    log("portal API from instance origin:", probe);
    await page.screenshot({ path: `${out}/04-instance.png`, fullPage: true });
    await ctx.storageState({ path: statePath("alice") });
  } else if (step === "admin-models") {
    // Approve the LunaRoute models (the only provider configured) and give
    // paid plans a monthly and daily credit allowance.
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await login(page, creds.admin_username, creds.admin_password);
    const result = await page.evaluate(async () => {
      const s = await (await fetch("/api/session")).json();
      const cur = await (await fetch("/api/admin/settings")).json();
      const available = cur.providerModels.map((m) => m.selector);
      const want = [
        { selector: "lunaroute/glm-5.3-flash", label: "GLM 5.3 Flash", thinking: ["off", "minimal", "low"], credits: 1 },
        { selector: "lunaroute/glm-5.3", label: "GLM 5.3", thinking: ["off", "minimal", "low", "medium"], credits: 3 },
        { selector: "lunaroute/deepseek-4.1-flash", label: "DeepSeek 4.1 Flash", thinking: ["low", "medium"], credits: 1 },
      ].filter((m) => available.includes(m.selector));
      const doc = { ...(cur.active?.document ?? cur.defaults) };
      doc.models = { allowed: want, tiers: { quick: "lunaroute/glm-5.3-flash", default: "lunaroute/glm-5.3", complex: "lunaroute/glm-5.3" } };
      // FRANCOIS-004: credits belong to plans; give every paid plan the
      // subscriber allowance unless it already has one.
      doc.plans = (doc.plans ?? []).map((p) => (p.key === "free" || p.monthlyCredits ? p : { ...p, monthlyCredits: 2000, dailyCredits: 200 }));
      const r = await fetch("/api/admin/settings", { method: "PATCH", headers: { "Content-Type": "application/json", "X-CSRF-Token": s.csrfToken }, body: JSON.stringify(doc) });
      return { status: r.status, body: await r.json(), available: available.length, modelProvider: cur.secrets?.modelProvider };
    });
    log("provider models", result.available, "modelProvider configured", result.modelProvider);
    log("save", result.status, JSON.stringify({ revision: result.body.revision ?? result.body.error, approved: result.body.document?.models?.allowed?.map((m) => m.selector) }));
    if (result.status !== 200) process.exitCode = 1;
  } else if (step === "alice-sample") {
    // A real model call from a subscriber's instance through the hosted
    // sampling runtime (Pi worker -> LunaRoute).
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await login(page, creds.alice_username, creds.alice_password);
    await page.goto(BASE + "/apps/polis");
    await page.getByRole("button", { name: /run in a new private instance/i }).click();
    await page.waitForURL(/\/instances\//, { timeout: 30000 });
    let appFrame;
    for (let i = 0; i < 30 && !appFrame; i++) {
      appFrame = page.frames().find((f) => /^https:\/\/i-[^.]+\.apps\.francois\.yolo\.scapegoat\.dev\//.test(f.url()));
      if (!appFrame) await page.waitForTimeout(1000);
    }
    if (!appFrame) throw new Error("no instance frame on the app origin");
    await page.waitForTimeout(4000);
    const res = await appFrame.evaluate(async () => {
      const t0 = Date.now();
      try {
        const sample = await window.claude.use("sample");
        const limits = await sample.limits().catch((e) => ({ error: String(e) }));
        const r = await sample([{ role: "user", content: "Reply with exactly the word: pong" }], { modelTier: "quick" });
        return { ok: true, ms: Date.now() - t0, text: String(r.text).slice(0, 80), tier: r.modelTierApplied, model: r.modelUsed ?? r.modelApplied, limits };
      } catch (e) {
        return { ok: false, ms: Date.now() - t0, error: `${e.code ?? e.name}: ${e.message}` };
      }
    });
    log("sample", JSON.stringify(res));
    await page.screenshot({ path: `${out}/06-sample-instance.png`, fullPage: true });
    if (!res.ok) process.exitCode = 1;
  } else if (step === "control") {
    // Without signing out, a live ZITADEL session makes /auth/login silent.
    // This is the behaviour the logout step must break.
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await login(page, creds.alice_username, creds.alice_password);
    await page.evaluate(async () => {
      const s = await (await fetch("/api/session")).json();
      await fetch("/auth/logout", { method: "POST", headers: { "X-CSRF-Token": s.csrfToken } }); // portal only; redirect not followed
    });
    log("portal session after API-only logout:", (await session(page)).authenticated);
    await page.goto(BASE + "/auth/login");
    await page.waitForLoadState("networkidle");
    const silent = page.url().startsWith(BASE);
    log("next /auth/login lands on", silent ? "Francois (silent re-login, as expected while ZITADEL session lives)" : new URL(page.url()).host + new URL(page.url()).pathname);
    log("authenticated:", (await session(page)).authenticated);
    await ctx.storageState({ path: statePath("alice") });
    if (!silent) process.exitCode = 1;
  } else if (step === "logout") {
    if (!existsSync(statePath("alice"))) throw new Error("run alice-subscribe first");
    const ctx = await browser.newContext({ storageState: statePath("alice") });
    const page = await ctx.newPage();
    await page.goto(BASE + "/");
    log("signed in before:", (await session(page)).authenticated);
    await page.getByRole("button", { name: "Sign out" }).click();
    await page.waitForURL((u) => u.href.startsWith(BASE + "/") , { timeout: 30000 });
    await page.waitForTimeout(2000);
    log("after sign-out at", page.url(), "authenticated:", (await session(page)).authenticated);
    await page.goto(BASE + "/auth/login");
    await page.waitForLoadState("networkidle");
    const silent = page.url().startsWith(BASE);
    log("next /auth/login lands on", silent ? "Francois (silent re-login: FAIL)" : new URL(page.url()).host + new URL(page.url()).pathname);
    await page.screenshot({ path: `${out}/05-after-logout-login.png`, fullPage: true });
    if (silent) process.exitCode = 1;
  }
} finally {
  await browser.close();
}
