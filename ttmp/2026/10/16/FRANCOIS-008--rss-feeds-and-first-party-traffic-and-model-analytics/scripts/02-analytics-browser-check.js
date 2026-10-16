// Run through Playwright's browser_run_code tool while 01-analytics-preview.mjs is in tmux.
// prettier-ignore
async (page) => {
  const checks = [],
    errors = [],
    requests = [];
  const check = (condition, description) => {
    if (!condition) throw new Error(description);
    checks.push(description);
  };
  page.on("pageerror", (error) => errors.push(String(error)));
  page.on("request", (request) => {
    if (request.url().includes("/api/admin/analytics?"))
      requests.push(new URL(request.url()));
  });
  const frame = () =>
    page.evaluate(
      () =>
        new Promise((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(resolve)),
        ),
    );
  const apply = async (action, predicate = () => true) => {
    const response = page.waitForResponse(
      (response) =>
        response.url().includes("/api/admin/analytics?") &&
        predicate(new URL(response.url())),
    );
    await action();
    await response;
    await frame();
  };
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("http://127.0.0.1:18908/admin/analytics");
  await page
    .getByRole("heading", { name: "Daily page views", exact: true })
    .waitFor();
  const initial = requests.length;
  await page
    .getByRole("button", { name: "Stack by page", exact: true })
    .click();
  await frame();
  check(
    requests.length === initial,
    "Switching stack mode does not query the server",
  );
  const aligned = await page
    .locator("svg[role=img] rect")
    .evaluateAll((rects) => {
      const edges = rects.map((rect) => {
        const stroke =
          rect.getAttribute("stroke") === "none"
            ? 0
            : Number(rect.getAttribute("stroke-width"));
        const x = Number(rect.getAttribute("x")),
          width = Number(rect.getAttribute("width"));
        return [x - stroke / 2, x + width + stroke / 2];
      });
      return edges.every((edge, i) =>
        edge.every(
          (value, k) => Math.abs(value - edges[i - (i % 4)][k]) < 0.01,
        ),
      );
    });
  check(
    aligned,
    "Every stacked segment has the same visible left and right edges",
  );
  check(
    await page
      .locator(".analytics-legend")
      .innerText()
      .then((text) => text.includes("Other pages")),
    "Stack includes Other pages",
  );
  await page
    .getByRole("button", { name: "Stack by referrer", exact: true })
    .click();
  await frame();
  check(
    await page
      .locator(".analytics-legend")
      .innerText()
      .then(
        (text) =>
          text.includes("Direct / unspecified") &&
          text.includes("Other referrers"),
      ),
    "Direct and Other referrers remain distinct",
  );
  const trigger = page.locator(".analytics-date-trigger"),
    dialog = page.getByRole("dialog");
  await trigger.click();
  await dialog.getByLabel("Start date", { exact: true }).fill("2026-12-31");
  await dialog
    .getByLabel("End date (included)", { exact: true })
    .fill("2026-12-30");
  await dialog
    .getByRole("button", { name: "Apply range", exact: true })
    .click();
  check(
    await dialog.getByRole("alert").isVisible(),
    "Inverted date range is rejected inline",
  );
  check(requests.length === initial, "Editing and invalid Apply do not query");
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  check(
    await trigger.evaluate((element) => element === document.activeElement),
    "Cancel restores date-trigger focus",
  );
  await trigger.click();
  await page.keyboard.press("Escape");
  check(
    !(await dialog.isVisible()) && requests.length === initial,
    "Escape closes the dialog without querying",
  );
  await trigger.click();
  await dialog
    .getByRole("button", { name: "Last 7 days", exact: true })
    .click();
  check(requests.length === initial, "Date presets remain drafts until Apply");
  await apply(() =>
    dialog.getByRole("button", { name: "Apply range", exact: true }).click(),
  );
  const dates = requests.at(-1).searchParams;
  check(
    (Date.parse(dates.get("to")) - Date.parse(dates.get("from"))) / 86400000 ===
      7,
    "Last seven days applies correct exclusive UTC boundary",
  );
  await page.getByLabel("Page", { exact: true }).fill("/apps");
  await page
    .getByLabel("Referrer filter", { exact: true })
    .selectOption("direct");
  await apply(
    () =>
      page.getByRole("button", { name: "Apply filters", exact: true }).click(),
    (url) => url.searchParams.has("referrer"),
  );
  check(
    requests.at(-1).searchParams.get("path") === "/apps" &&
      requests.at(-1).searchParams.has("referrer") &&
      requests.at(-1).searchParams.get("referrer") === "",
    "Direct filter sends explicit empty referrer and exact path",
  );
  await page
    .getByRole("button", { name: "Clear traffic filters", exact: true })
    .click();
  // Clearing can reuse RTK Query's cached response; no fresh request is required.
  await frame();
  check(
    (await page.getByLabel("Page", { exact: true }).inputValue()) === "",
    "Clear resets applied path and referring hostname",
  );
  await page.getByRole("button", { name: "Model usage", exact: true }).click();
  await page
    .getByRole("heading", { name: "Usage by artifact", exact: true })
    .waitFor();
  check(
    (await page.locator("svg[role=img]").count()) === 2,
    "Model usage has token and cost graphs",
  );
  check(
    await page.getByText("Unknown artifact", { exact: true }).isVisible(),
    "Unattributed historical usage remains visible",
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await frame();
  check(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "Model graphs and tables fit mobile without page overflow",
  );
  await trigger.click();
  await frame();
  check(
    await dialog.evaluate(
      (element) =>
        element.getBoundingClientRect().left >= 0 &&
        element.getBoundingClientRect().right <= innerWidth,
    ),
    "Date dialog fits mobile viewport",
  );
  await dialog.getByRole("button", { name: "Close", exact: true }).click();
  await page.getByRole("button", { name: "Traffic", exact: true }).click();
  await frame();
  check(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "Traffic controls and stacks fit mobile",
  );
  check(errors.length === 0, "No browser page errors");
  return { checks, requests: requests.map((url) => url.search), errors };
}
