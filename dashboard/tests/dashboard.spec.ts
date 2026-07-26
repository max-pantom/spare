import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const machine = {
  id: "spare_test",
  hostname: "studio-mac",
  os: "darwin",
  architecture: "arm64",
  logicalCores: 8,
  memoryTotalBytes: 17179869184,
  storageAvailableBytes: 214748364800,
  lanAddresses: ["192.168.1.24"],
  initializedAt: "2026-07-26T10:00:00Z",
  lastProfiledAt: "2026-07-26T10:00:00Z"
};

const healthySite = {
  id: "site",
  recipeId: "site",
  mode: "installed",
  desiredState: "running",
  status: "healthy",
  rootPath: "/Users/max/Prototype",
  port: 7340,
  portMode: "auto",
  urls: [
    "http://127.0.0.1:7340",
    "http://192.168.1.24:7340",
    "http://studio-mac.local:7340"
  ],
  createdAt: "2026-07-26T10:00:00Z",
  updatedAt: "2026-07-26T10:00:02Z"
};

async function mockDashboard(page: import("@playwright/test").Page, instances = [healthySite]) {
  await page.route("**/api/v1/machine", (route) =>
    route.fulfill({ json: machine })
  );
  await page.route("**/api/v1/instances", (route) =>
    route.fulfill({ json: instances })
  );
  await page.route("**/api/v1/instances/site/stop", (route) =>
    route.fulfill({
      json: { ...healthySite, status: "stopped", desiredState: "stopped" }
    })
  );
}

test("shows the current job and passes automated accessibility checks", async ({
  page
}) => {
  await mockDashboard(page);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "This computer is a Site" })
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /Open site/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "Stop site" })).toBeVisible();

  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});

test("empty state points to the first CLI action", async ({ page }) => {
  await mockDashboard(page, []);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "This computer is ready" })
  ).toBeVisible();
  await expect(page.getByText("spare try site ./public")).toBeVisible();
});

test("reflows at 320 pixels and controls remain keyboard reachable", async ({
  page
}) => {
  await page.setViewportSize({ width: 320, height: 800 });
  await mockDashboard(page);
  await page.goto("/");

  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#main")).toBeVisible();

  const hasHorizontalOverflow = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth
  );
  expect(hasHorizontalOverflow).toBe(false);
});

