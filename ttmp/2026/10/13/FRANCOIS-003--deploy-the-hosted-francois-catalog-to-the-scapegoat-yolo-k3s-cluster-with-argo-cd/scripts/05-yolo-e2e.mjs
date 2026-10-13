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
    await page.goto(BASE + "/account");
    await page.getByRole("button", { name: /subscribe/i }).first().click();
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
