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
  capabilities: {
    canServeLAN: true,
    canRunPersistent: true,
    canStoreLargeFiles: true,
    canRunContainers: false,
    hasBattery: true,
    hasExternalStorage: false
  },
  initializedAt: "2026-07-26T10:00:00Z",
  lastProfiledAt: "2026-07-26T10:00:00Z"
};

const recipes = [
  {
    id: "drop",
    title: "Drop",
    version: "0.1.0",
    description: "Send files to this computer from a browser on the local network.",
    runtime: "native",
    supportedSystems: ["darwin", "windows", "linux"],
    resources: {
      memoryRecommendedBytes: 134217728,
      memoryMaximumBytes: 536870912,
      cpuMaximum: 1
    },
    config: [],
    permissions: [
      {
        id: "network.local",
        description: "Accept connections from your local network",
        granted: true
      },
      {
        id: "network.internet",
        description: "Access internet services",
        granted: false
      }
    ],
    compatibility: {
      supported: true,
      rating: "Excellent",
      reasons: ["This computer has enough memory."],
      warnings: ["This computer may sleep when its lid is closed."]
    }
  },
  {
    id: "site",
    title: "Site",
    version: "0.1.0",
    description: "Serve a folder as a read-only website on the local network.",
    runtime: "native",
    supportedSystems: ["darwin", "windows", "linux"],
    resources: {
      memoryRecommendedBytes: 67108864,
      memoryMaximumBytes: 268435456,
      cpuMaximum: 1
    },
    config: [],
    permissions: [
      {
        id: "filesystem.read",
        description: "Read files in the folder you select",
        granted: true
      }
    ],
    compatibility: {
      supported: true,
      rating: "Excellent",
      reasons: ["This computer has enough memory."],
      warnings: []
    }
  },
  {
    id: "hook",
    title: "Hook",
    version: "0.1.0",
    description:
      "Receive, inspect, and replay webhook requests on the local network.",
    runtime: "native",
    supportedSystems: ["darwin", "windows", "linux"],
    resources: {
      memoryRecommendedBytes: 67108864,
      memoryMaximumBytes: 268435456,
      cpuMaximum: 1
    },
    config: [],
    permissions: [
      {
        id: "network.local",
        description: "Accept connections from your local network",
        granted: true
      },
      {
        id: "network.internet",
        description: "Access internet services",
        granted: true
      }
    ],
    compatibility: {
      supported: true,
      rating: "Excellent",
      reasons: ["This computer has enough memory."],
      warnings: []
    }
  }
];

const healthySite = {
  id: "site",
  recipeId: "site",
  version: "0.1.0",
  runtime: "native",
  mode: "installed",
  desiredState: "running",
  status: "healthy",
  rootPath: "/Users/max/Prototype",
  dataPath: "/Users/max/Prototype",
  config: { path: "/Users/max/Prototype" },
  port: 7340,
  portMode: "auto",
  urls: [
    "http://127.0.0.1:7340",
    "http://192.168.1.24:7340",
    "http://studio-mac.local:7340"
  ],
  storageAvailableBytes: 0,
  itemCount: 0,
  createdAt: "2026-07-26T10:00:00Z",
  updatedAt: "2026-07-26T10:00:02Z"
};

const healthyDrop = {
  ...healthySite,
  id: "drop",
  recipeId: "drop",
  rootPath: "/Users/max/Received",
  dataPath: "/Users/max/Received",
  config: {
    destination: "/Users/max/Received",
    "max-file-size": 2000000000
  },
  itemCount: 7,
  storageAvailableBytes: 107374182400
};

const healthyHook = {
  ...healthySite,
  id: "hook",
  recipeId: "hook",
  rootPath: "",
  dataPath: "",
  config: {},
  itemCount: 4,
  storageAvailableBytes: 0
};

const events = [
  {
    id: 1,
    instanceId: "site",
    level: "info",
    kind: "instance_healthy",
    message: "Site is ready.",
    createdAt: "2026-07-26T10:00:02Z"
  }
];

async function mockDashboard(
  page: import("@playwright/test").Page,
  instances = [healthySite],
  activity = events
) {
  await page.route("**/api/v1/machine", (route) =>
    route.fulfill({ json: machine })
  );
  await page.route("**/api/v1/recipes", (route) =>
    route.fulfill({ json: recipes })
  );
  await page.route("**/api/v1/events*", (route) =>
    route.fulfill({ json: activity })
  );
  await page.route("**/api/v1/instances", (route) =>
    route.fulfill({ json: instances })
  );
  await page.route("**/api/v1/instances/*/stop", (route) => {
    const current = instances[0] ?? healthySite;
    return route.fulfill({
      json: { ...current, status: "stopped", desiredState: "stopped" }
    });
  });
}

test("shows reusable instance, recipe, machine, and activity views", async ({
  page
}) => {
  await mockDashboard(page);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "This computer is a Site" })
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /Open Site/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "Stop Site" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Recipes" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Machine" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Activity" })).toBeVisible();
  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/dashboard.png",
      fullPage: true
    });
  }

  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});

test("shows Drop files and storage without Site-specific copy", async ({
  page
}) => {
  await mockDashboard(page, [healthyDrop], [
    { ...events[0], instanceId: "drop", message: "Drop is ready." }
  ]);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "This computer is a Drop" })
  ).toBeVisible();
  await expect(page.getByText("Files received")).toBeVisible();
  await expect(page.getByText("7", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: /Open Drop/ })).toBeVisible();
});

test("shows Hook request metrics without selected-folder details", async ({
  page
}) => {
  await mockDashboard(page, [healthyHook], [
    { ...events[0], instanceId: "hook", message: "Hook is ready." }
  ]);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "This computer is a Hook" })
  ).toBeVisible();
  await expect(page.getByText("Requests received")).toBeVisible();
  await expect(page.getByText("4", { exact: true })).toBeVisible();
  await expect(page.getByText("Hook is ready to receive")).toBeVisible();
  await page.getByText("Show instance details").click();
  await expect(page.getByText("Folder", { exact: true })).toHaveCount(0);
});

test("empty state points to all first recipe actions", async ({ page }) => {
  await mockDashboard(page, [], []);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "This computer is ready" })
  ).toBeVisible();
  const firstActions = page.getByLabel("Try a recipe");
  await expect(firstActions.getByText("spare try site ./public")).toBeVisible();
  await expect(
    firstActions.getByText("spare try drop ./received-files")
  ).toBeVisible();
  await expect(firstActions.getByText("spare try hook")).toBeVisible();
});

test("reflows at 320 pixels and controls remain keyboard reachable", async ({
  page
}) => {
  await page.setViewportSize({ width: 320, height: 800 });
  await mockDashboard(page);
  await page.goto("/");

  await page.locator("body").click({ position: { x: 1, y: 1 } });
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#main")).toBeVisible();

  const hasHorizontalOverflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(hasHorizontalOverflow).toBe(false);
  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/dashboard-mobile.png",
      fullPage: true
    });
  }
});

test("survives 200 percent text scaling", async ({ page }) => {
  await page.setViewportSize({ width: 640, height: 800 });
  await mockDashboard(page);
  await page.goto("/");
  await page.evaluate(() => {
    document.documentElement.style.fontSize = "200%";
  });

  await expect(
    page.getByRole("heading", { name: "This computer is a Site" })
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /Open Site/ })).toBeVisible();
  const hasHorizontalOverflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(hasHorizontalOverflow).toBe(false);
});
