// Snippets for the Playwright MCP tool browser_run_code_unsafe (each is the
// body of `async (page) => { … }`). Screenshots must be written under the
// repository's .playwright-mcp/ directory (the tool's allowed roots); move
// them to a scratch directory afterwards. .playwright-mcp/ is git-ignored.

// 1. A hi-DPI context (crisp 2x shots) for a complex UI at desktop size.
const ctx = await page.context().browser().newContext({ viewport: { width: 1920, height: 1200 }, deviceScaleFactor: 2 });
const p = await ctx.newPage();
await p.goto('http://127.0.0.1:8765/app.html'); await p.waitForTimeout(5000);
await p.screenshot({ path: '/ABS/REPO/.playwright-mcp/shots/app/ui.png' });

// 2. Toggle panels by their pressed state (Living Comic / Hypercore style).
const tog = async (n, on) => { const b = p.locator(`button.tog:has-text("${n}")`).first();
  if ((await b.getAttribute('aria-pressed') === 'true') !== on) await b.click(); };

// 3. Pick a preset from a "▾" menu, wait for jobs, return to the main tab.
await p.getByText(/▾/).first().click(); await p.waitForTimeout(500);
await p.getByText('The Night Market of Ulan Tor', { exact: true }).first().click(); await p.waitForTimeout(12000);
await p.getByText('Spread', { exact: true }).first().click();

// 4. Element screenshot (the comic page, a stage canvas) with a tall viewport
//    so the element is not clipped by its scroll container.
await p.setViewportSize({ width: 1920, height: 2600 });
await p.locator('.page').first().screenshot({ path: '/ABS/REPO/.playwright-mcp/shots/app/page.png' });

// 5. Clip to the union of several elements (e.g. zine pages drawn as canvases).
const boxes = await p.$$eval('canvas.ovl', cs => cs.map(c => c.getBoundingClientRect()).filter(r => r.width > 50).map(r => [r.x, r.y, r.right, r.bottom]));
const [x0, y0] = [Math.min(...boxes.map(b => b[0])), Math.min(...boxes.map(b => b[1]))];
const [x1, y1] = [Math.max(...boxes.map(b => b[2])), Math.max(...boxes.map(b => b[3]))];
await p.screenshot({ path: '/ABS/REPO/.playwright-mcp/shots/app/spread.png', clip: { x: x0, y: y0, width: x1 - x0, height: y1 - y0 } });

// 6. Mid-animation frames: trigger, then shoot a few times.
await p.getByRole('button', { name: 'Kickflip', exact: true }).click();
for (const ms of [350, 250, 250]) { await p.waitForTimeout(ms); await p.screenshot({ path: `/ABS/REPO/.playwright-mcp/shots/app/trick-${Date.now()}.png` }); }

await ctx.close();

// 7. Verify a published page: every image answers 200.
await page.goto('https://slop.boutique/apps/SLUG');
return await page.evaluate(async () => { const s = [...document.querySelectorAll('img')].map(i => i.currentSrc || i.src);
  const st = await Promise.all(s.map(u => fetch(u).then(r => r.status))); return [s.length, st.filter(x => x !== 200).length]; });
