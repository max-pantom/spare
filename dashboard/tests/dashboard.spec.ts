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
    config: [
      {
        id: "destination",
        type: "directory",
        label: "Destination folder",
        description: "Files received through Drop are written here.",
        required: true
      },
      {
        id: "max-file-size",
        type: "size",
        label: "Maximum file size",
        description: "Drop rejects individual files larger than this limit.",
        required: false,
        default: "2GB"
      }
    ],
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
    config: [
      {
        id: "path",
        type: "directory",
        label: "Site folder",
        description: "The folder Spare will serve read-only.",
        required: true
      }
    ],
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

const systemHistory = [
  {
    id: 5,
    instanceId: "drop",
    level: "info",
    kind: "instance_started",
    message: "Drop started.",
    createdAt: "2026-07-26T10:05:00Z"
  },
  {
    id: 4,
    instanceId: "drop",
    level: "info",
    kind: "drop_file_received",
    message: "campaign.zip received.",
    details: { itemName: "campaign.zip", deviceName: "iPhone" },
    createdAt: "2026-07-26T10:04:00Z"
  },
  {
    id: 3,
    instanceId: "drop",
    level: "info",
    kind: "address_changed",
    message: "Address changed.",
    createdAt: "2026-07-26T10:03:00Z"
  },
  {
    id: 2,
    instanceId: "drop",
    level: "warning",
    kind: "worker_recovered",
    message: "Drop recovered.",
    createdAt: "2026-07-26T10:02:00Z"
  },
  {
    id: 1,
    instanceId: "drop",
    level: "warning",
    kind: "storage_disconnected",
    message: "Storage disconnected.",
    createdAt: "2026-07-26T10:01:00Z"
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
  await mockDashboard(page, [healthySite], systemHistory);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "studio mac", exact: true })
  ).toBeVisible();
  await expect(
    page.locator(".dashboard-remote-machine > strong")
  ).toHaveText(
    "studio mac"
  );
  await expect(
    page.locator(".dashboard-remote-machine > strong")
  ).toHaveCSS(
    "text-transform",
    "uppercase"
  );
  await expect(page.getByText("Connected locally", { exact: true })).toBeVisible();
  const dashboardNavigation = page.getByRole("navigation", {
    name: "Dashboard sections"
  });
  await expect
    .poll(() =>
      dashboardNavigation.locator(".dashboard-nav-icon").evaluateAll((icons) =>
        icons.map((icon) => ({
          icon: icon.getAttribute("data-icon"),
          viewBox: icon.getAttribute("viewBox")
        }))
      )
    )
    .toEqual([
      { icon: "home", viewBox: "0 0 12 12" },
      { icon: "transfer", viewBox: "0 0 12 12" },
      { icon: "jobs", viewBox: "0 0 12 12" },
      { icon: "computer", viewBox: "0 0 12 12" },
      { icon: "activity", viewBox: "0 0 12 12" }
    ]);
  await dashboardNavigation
    .getByRole("link", { name: "Transfer", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "studio mac is a Site" })
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /Open Site/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "Stop Site" })).toBeVisible();
  await dashboardNavigation
    .getByRole("link", { name: "Jobs", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Jobs" })).toBeVisible();
  await dashboardNavigation
    .getByRole("link", { name: "Computer", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Computer", exact: true })
  ).toBeVisible();
  const browserTechnicalDetails = page.locator(".machine-details-section");
  await expect(
    browserTechnicalDetails.getByRole("heading", { name: "Computer details" })
  ).toBeVisible();
  await expect(
    browserTechnicalDetails.locator(".machine-detail-grid > div")
  ).toHaveCount(10);
  await expect(page.locator("#machine details")).toHaveCount(0);
  for (const detail of [
    "CPU",
    "Memory",
    "Available system storage",
    "Network",
    "Battery",
    "Architecture",
    "External drives",
    "Container support"
  ]) {
    await expect(
      browserTechnicalDetails.getByText(detail, { exact: true })
    ).toBeVisible();
  }
  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/dashboard-computer.png",
      fullPage: true
    });
  }
  await dashboardNavigation
    .getByRole("link", { name: "Activity", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Activity" })).toBeVisible();
  for (const historyItem of [
    "Drop started",
    "campaign.zip received from iPhone",
    "Local address changed",
    "Drop recovered after closing unexpectedly",
    "Storage folder disconnected"
  ]) {
    await expect(page.getByText(historyItem, { exact: true })).toBeVisible();
  }
  await expect(
    page.getByRole("heading", { name: "Technical details" })
  ).toHaveCount(0);
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
  await page.goto("/#transfer");

  await expect(
    page.getByRole("heading", { name: "studio mac is a Drop" })
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
  await page.goto("/#transfer");

  await expect(
    page.getByRole("heading", { name: "studio mac is a Hook" })
  ).toBeVisible();
  await expect(page.getByText("Requests received")).toBeVisible();
  await expect(page.getByText("4", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Hook on studio-mac is ready to receive")
  ).toBeVisible();
  await page.getByText("Show instance details").click();
  await expect(page.getByText("Folder", { exact: true })).toHaveCount(0);
});

test("explains repeated failures and what Spare already tried", async ({
  page
}) => {
  await mockDashboard(
    page,
    [
      {
        ...healthyDrop,
        status: "failed",
        problem: {
          code: "restart_limit_reached",
          severity: "error",
          summary: "Drop stopped after repeatedly failing.",
          recovery: "Check the job details before starting it again."
        }
      }
    ],
    []
  );
  await page.goto("/#transfer");

  await expect(
    page.getByRole("heading", { name: "Drop stopped unexpectedly." })
  ).toBeVisible();
  await expect(
    page.getByText(
      "Spare tried to restart it five times, but it continues to fail."
    )
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Run repair" })).toBeVisible();
  await page.getByRole("button", { name: "View details" }).click();
  await expect(page.getByText("Drop 0.1.0")).toBeVisible();
  await expect(page.getByRole("button", { name: "Stop Drop" })).toBeVisible();

  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/dashboard-failure.png",
      fullPage: true
    });
  }
});

test("empty state points to all first recipe actions", async ({ page }) => {
  await mockDashboard(page, [], []);
  await page.goto("/#transfer");

  await expect(
    page.getByRole("heading", { name: "studio-mac is ready" })
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
  await mockDashboard(page, [healthyDrop]);
  await page.goto("/#transfer");

  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#main")).toBeVisible();
  await page.evaluate(() => {
    window.location.hash = "transfer";
  });
  await expect(
    page.getByRole("heading", { name: "Send files to studio mac" })
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /Choose files/ })).toBeVisible();
  await expect(page.getByText("Connected locally", { exact: true })).toBeVisible();
  const mobileMenu = page.locator(".dashboard-mobile-menu summary");
  await expect(mobileMenu).toHaveText("Menu");
  await expect(mobileMenu).toBeVisible();
  await mobileMenu.focus();
  await expect(mobileMenu).toBeFocused();

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
  await page.goto("/#transfer");
  await page.evaluate(() => {
    document.documentElement.style.fontSize = "200%";
  });

  await expect(
    page.getByRole("heading", { name: "studio mac is a Site" })
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /Open Site/ })).toBeVisible();
  const hasHorizontalOverflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(hasHorizontalOverflow).toBe(false);
});

test("desktop loading stays inside the dark application shell", async ({
  page
}) => {
  await page.addInitScript(() => {
    const pending = new Promise(() => undefined);
    Object.defineProperty(window, "go", {
      configurable: true,
      value: {
        desktop: {
          App: {
            Bootstrap: () => pending,
            Snapshot: () => pending
          }
        }
      }
    });
  });
  await page.setViewportSize({ width: 930, height: 509 });
  await page.goto("/");
  await expect(page.getByText("Starting Spare", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Connecting to the background service", { exact: true })
  ).toBeVisible();
  await expect(page.locator("body")).toHaveCSS(
    "background-color",
    "rgb(28, 28, 28)"
  );
  const loadingStatus = await page.locator(".desktop-ready").boundingBox();
  const loadingCard = await page.locator(".desktop-service-card").boundingBox();
  expect(loadingStatus?.x).toBe(loadingCard?.x);
});

test("desktop startup failure uses the aligned application shell", async ({
  page
}) => {
  await page.addInitScript(() => {
    const unavailable = () =>
      Promise.reject(new Error("Background service unavailable"));
    Object.defineProperty(window, "go", {
      configurable: true,
      value: {
        desktop: {
          App: {
            Bootstrap: unavailable,
            Snapshot: unavailable,
            Repair: unavailable
          }
        }
      }
    });
  });
  await page.setViewportSize({ width: 930, height: 509 });
  await page.goto("/");
  await expect(page.locator(".desktop-ready")).toContainText(
    "Spare could not start"
  );
  await expect(page.getByRole("button", { name: "Run repair" })).toBeVisible();
  await expect(
    page.getByText("Background service unavailable", { exact: true })
  ).toBeVisible();
});

test("desktop activity handles a first-run empty history", async ({ page }) => {
  await page.addInitScript(
    ({ machine, recipes }) => {
      const snapshot = {
        surface: "desktop",
        machine,
        recipes,
        instances: [],
        events: null,
        preferences: {
          theme: "dark",
          notifications: true,
          recipeNotifications: {
            drop: true,
            site: true,
            hook: true
          },
          openAfterLogin: false,
          showInMenuBar: true,
          keepRecipesRunningAfterLogin: true
        }
      };
      Object.defineProperty(window, "go", {
        configurable: true,
        value: {
          desktop: {
            App: {
              Bootstrap: async () => snapshot,
              Snapshot: async () => snapshot,
              PendingLaunchPaths: async () => []
            }
          }
        }
      });
    },
    { machine, recipes }
  );
  await page.setViewportSize({ width: 930, height: 509 });
  await page.goto("/");
  await page.getByRole("button", { name: "Activity", exact: true }).click();
  await expect(page.getByText("No activity yet", { exact: true })).toBeVisible();
  await expect(page.locator(".desktop-root")).toBeVisible();
});

test("desktop first launch sets up Drop without the CLI", async ({ page }) => {
  await page.addInitScript(
    ({ machine, recipes, healthyDrop }) => {
      let instances: typeof healthyDrop[] = [];
      const events: Array<Record<string, unknown>> = [];
      const snapshot = () => ({
        surface: "desktop",
        machine,
        recipes,
        instances,
        events,
        preferences: {
          theme: "dark",
          notifications: true,
          recipeNotifications: {
            drop: true,
            site: true,
            hook: true
          },
          openAfterLogin: false,
          showInMenuBar: true,
          keepRecipesRunningAfterLogin: true
        }
      });
      window.go = {
        desktop: {
          App: {
            Bootstrap: async () => snapshot(),
            Snapshot: async () => snapshot(),
            CreateInstance: async (input) => {
              const created = {
                ...healthyDrop,
                mode: input.mode,
                rootPath: String(input.config.destination),
                dataPath: String(input.config.destination),
                config: input.config
              };
              instances = [created];
              events.unshift({
                id: 10,
                instanceId: "drop",
                level: "info",
                kind: "instance_created",
                message: "Drop started.",
                createdAt: "2026-07-26T10:00:02Z"
              });
              return created;
            },
            ConfigureInstance: async (_id, input) => {
              instances = [
                {
                  ...instances[0],
                  rootPath: String(input.config.destination),
                  dataPath: String(input.config.destination),
                  config: input.config
                }
              ];
              return instances[0];
            },
            StartInstance: async () => {
              instances = [
                {
                  ...instances[0],
                  status: "healthy",
                  desiredState: "running"
                }
              ];
              return instances[0];
            },
            StopInstance: async () => {
              instances = [
                {
                  ...instances[0],
                  status: "stopped",
                  desiredState: "stopped"
                }
              ];
              return instances[0];
            },
            PromoteInstance: async () => ({
              ...instances[0],
              mode: "installed"
            }),
            RemoveInstance: async () => {
              instances = [];
            },
            Repair: async () => snapshot(),
            SavePreferences: async () => undefined,
            ChooseDirectory: async () => "/Users/max/Downloads/Spare",
            ChooseFile: async () => "",
            ChooseFiles: async () => ["/Users/max/Desktop/report.pdf"],
            DescribeDroppedPaths: async () => [],
            PendingLaunchPaths: async () => [],
            OpenRecipePackage: async () => undefined,
            AddDropFiles: async (_instanceId, paths) => {
              instances = [
                {
                  ...instances[0],
                  itemCount: instances[0].itemCount + paths.length
                }
              ];
              events.unshift({
                id: 11,
                instanceId: "drop",
                level: "info",
                kind: "drop_file_received",
                message: "report.pdf was received.",
                details: { count: 1, itemName: "report.pdf" },
                createdAt: "2026-07-26T10:00:03Z"
              });
              return ["report.pdf"];
            },
            ExportBackup: async () => "/Users/max/Desktop/drop.spare-backup",
            RestoreBackup: async () => instances[0],
            OpenURL: async () => undefined,
            OpenDashboard: async () => undefined,
            RevealPath: async () => undefined,
            RevealReceivedFile: async () => undefined,
            Uninstall: async () => undefined,
            Quit: async () => undefined
          }
        }
      };
    },
    { machine, recipes, healthyDrop }
  );

  await page.goto("/");
  const sidebar = page.getByRole("navigation", { name: "Primary" });
  await expect(sidebar.getByRole("button", { name: "Home" })).toBeVisible();
  await expect(sidebar.getByRole("button", { name: "Jobs" })).toBeVisible();
  await expect(sidebar.getByRole("button", { name: "Activity" })).toBeVisible();
  await expect(sidebar.getByRole("button", { name: "Computer" })).toBeVisible();
  await expect(sidebar.getByRole("button", { name: "Settings" })).toBeVisible();
  await expect(sidebar.locator(".nav-icon")).toHaveCount(5);
  await expect
    .poll(() =>
      page.locator(".desktop-root").evaluate((root) => {
        const styles = getComputedStyle(root);
        return {
          bodyFontSize: styles
            .getPropertyValue("--desktop-body-font-size")
            .trim(),
          rowGap: styles.getPropertyValue("--desktop-sidebar-row-gap").trim(),
          iconTextGap: styles
            .getPropertyValue("--desktop-sidebar-icon-text-gap")
            .trim(),
          fontSize: styles
            .getPropertyValue("--desktop-sidebar-font-size")
            .trim(),
          fontWeight: styles
            .getPropertyValue("--desktop-sidebar-font-weight")
            .trim(),
          activeFontWeight: styles
            .getPropertyValue("--desktop-sidebar-active-font-weight")
            .trim(),
          iconSize: styles
            .getPropertyValue("--desktop-sidebar-icon-size")
            .trim()
        };
      })
    )
    .toEqual({
      bodyFontSize: "14px",
      rowGap: "2px",
      iconTextGap: "4px",
      fontSize: "14px",
      fontWeight: "400",
      activeFontWeight: "500",
      iconSize: "14px"
    });
  await expect(page.locator(".desktop-sidebar")).toHaveCSS("width", "124px");
  await expect(sidebar).toHaveCSS("width", "104px");
  await expect(sidebar).toHaveCSS("gap", "2px");
  await expect(
    sidebar.getByRole("button", { name: "Computer" })
  ).toHaveCSS("min-height", "24px");
  await expect(
    sidebar.getByRole("button", { name: "Jobs" })
  ).toHaveCSS("opacity", "1");
  await expect(sidebar.locator(".nav-icon").first()).toHaveCSS("width", "14px");
  await expect
    .poll(() =>
      sidebar.locator(".nav-icon").evaluateAll((icons) =>
        icons.map((icon) => ({
          icon: icon.getAttribute("data-icon"),
          tagName: icon.tagName,
          viewBox: icon.getAttribute("viewBox")
        }))
      )
    )
    .toEqual([
      { icon: "home", tagName: "svg", viewBox: "0 0 12 12" },
      { icon: "jobs", tagName: "svg", viewBox: "0 0 12 12" },
      { icon: "activity", tagName: "svg", viewBox: "0 0 12 12" },
      { icon: "computer", tagName: "svg", viewBox: "0 0 12 12" },
      { icon: "settings", tagName: "svg", viewBox: "0 0 12 12" }
    ]);
  await sidebar
    .getByRole("button", { name: "Settings", exact: true })
    .click();
  await page.evaluate(() => document.fonts.ready);
  const topTextAlignment = await page.evaluate(() => {
    const home = document.querySelector(".desktop-nav button");
    const pageLabel = document.querySelector(
      ".desktop-page-heading .eyebrow"
    );
    if (!home || !pageLabel) {
      return { home: Number.NaN, page: Number.NaN };
    }
    const homeBox = home.getBoundingClientRect();
    const pageBox = pageLabel.getBoundingClientRect();
    return {
      home: homeBox.top + homeBox.height / 2,
      page: pageBox.top + pageBox.height / 2
    };
  });
  expect(Number.isFinite(topTextAlignment.home)).toBe(true);
  expect(Number.isFinite(topTextAlignment.page)).toBe(true);
  expect(
    Math.abs(topTextAlignment.home - topTextAlignment.page)
  ).toBeLessThanOrEqual(0.5);
  const measureHeader = (selector: string) =>
    page.locator(selector).first().evaluate((header) => {
      const box = header.getBoundingClientRect();
      const children = Array.from(header.children).map((child) =>
        child.getBoundingClientRect()
      );
      return {
        x: box.x,
        width: box.width,
        height: box.height,
        gaps: children.slice(1).map((child, index) =>
          Number((child.top - children[index].bottom).toFixed(3))
        )
      };
    });
  const standardHeaders = [];
  for (const destination of ["Activity", "Computer", "Settings"]) {
    await sidebar
      .getByRole("button", { name: destination, exact: true })
      .click();
    standardHeaders.push(await measureHeader(".desktop-page-heading"));
  }
  expect(standardHeaders.map((header) => header.x)).toEqual([153, 153, 153]);
  expect(standardHeaders.map((header) => header.width)).toEqual([
    672, 672, 672
  ]);
  expect(standardHeaders.map((header) => header.gaps)).toEqual([
    [8, 8],
    [8, 8],
    [8, 8]
  ]);
  expect(
    Math.max(...standardHeaders.map((header) => header.height)) -
      Math.min(...standardHeaders.map((header) => header.height))
  ).toBeLessThanOrEqual(0.5);
  await sidebar.getByRole("button", { name: "Jobs", exact: true }).click();
  const jobsHeader = await measureHeader(".desktop-jobs-heading");
  const jobsCards = await page.locator(".desktop-job-cards").boundingBox();
  expect(jobsHeader.x).toBe(standardHeaders[0].x);
  expect(jobsHeader.width).toBe(standardHeaders[0].width);
  expect(jobsHeader.gaps).toEqual([8]);
  expect(jobsCards?.x).toBe(jobsHeader.x);
  await sidebar.getByRole("button", { name: "Home", exact: true }).click();
  await expect(page.getByText("Ready for a job", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Ready for a job", { exact: true })
  ).toHaveCSS("font-size", "14px");
  await expect(
    page.getByRole("heading", { name: "Give this computer a job." })
  ).toHaveCount(0);
  await expect(page.locator(".ready-grid, .ready-card, .recipe-icon")).toHaveCount(
    0
  );
  await sidebar
    .getByRole("button", { name: "Activity", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Activity", exact: true })
  ).toBeVisible();
  await expect(page.getByText("No activity yet", { exact: true })).toBeVisible();
  await sidebar.getByRole("button", { name: "Home", exact: true }).click();
  await sidebar.getByRole("button", { name: "Jobs" }).click();
  const desktopJobs = page.locator(".desktop-jobs-page");
  await expect(
    desktopJobs.getByRole("heading", { name: "Installed Jobs" })
  ).toBeVisible();
  await expect(desktopJobs.locator('input[type="search"]')).toHaveCount(0);
  await expect(desktopJobs.locator(".desktop-job-card h2")).toHaveText([
    "Drop",
    "Hook",
    "Site"
  ]);
  await expect(desktopJobs.locator(".desktop-job-card > p").first()).toHaveCSS(
    "font-size",
    "14px"
  );
  await expect(desktopJobs.locator(".desktop-job-card-action")).toHaveText([
    "Start",
    "Open",
    "Start"
  ]);
  await expect(
    desktopJobs.getByRole("button", { name: "Install more jobs" })
  ).toBeVisible();
  await sidebar.getByRole("button", { name: "Computer" }).click();
  await expect(page.getByRole("heading", { name: machine.hostname })).toBeVisible();
  const desktopTechnicalDetails = page.locator(".machine-technical-section");
  await expect(
    desktopTechnicalDetails.getByRole("heading", {
      name: "Technical details"
    })
  ).toBeVisible();
  for (const detail of [
    "CPU",
    "Memory",
    "Storage",
    "Network",
    "Battery",
    "Architecture",
    "External drives",
    "Container support"
  ]) {
    await expect(
      desktopTechnicalDetails.getByText(detail, { exact: true })
    ).toBeVisible();
  }
  await expect(desktopTechnicalDetails.locator(".technical-value")).toHaveCSS(
    "font-family",
    /Geist Mono Variable/
  );
  await expect(page.locator("#desktop-main")).toBeFocused();
  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/desktop-computer.png"
    });
  }
  await sidebar.getByRole("button", { name: "Home" }).click();
  await page.getByRole("button", { name: "Try Drop" }).click();
  const jobDetail = page.locator(".job-detail-page");
  await expect(
    jobDetail.getByRole("heading", {
      name: "Turn this computer into a nearby file receiver."
    })
  ).toBeVisible();
  await expect(
    jobDetail.getByText(
      "Send files from phones, tablets, and other computers without uploading them to the cloud."
    )
  ).toBeVisible();
  await expect(
    jobDetail.getByRole("heading", { name: "What it does" })
  ).toBeVisible();
  for (const benefit of [
    "Receives files over the local network",
    "Stores them in a folder you choose",
    "Shows recent transfers",
    "Works from any nearby browser"
  ]) {
    await expect(jobDetail.getByText(benefit, { exact: true })).toBeVisible();
  }
  await expect(
    jobDetail.getByRole("heading", { name: "This computer", exact: true })
  ).toBeVisible();
  await expect(jobDetail.getByText("Excellent for Drop")).toBeVisible();
  await expect(jobDetail.getByText("215 GB available")).toBeVisible();
  await expect(jobDetail.getByText("Local network available")).toBeVisible();
  await expect(jobDetail.getByText("Battery powered")).toBeVisible();
  await expect(
    jobDetail.getByRole("heading", { name: "Drop needs access to" })
  ).toBeVisible();
  for (const permission of [
    "Receive local network connections",
    "Write into Downloads/Spare",
    "Run after login"
  ]) {
    await expect(jobDetail.getByText(permission, { exact: true })).toBeVisible();
  }
  const jobDetailA11y = await new AxeBuilder({ page })
    .include(".job-detail-page")
    .analyze();
  expect(jobDetailA11y.violations).toEqual([]);
  await page.setViewportSize({ width: 320, height: 800 });
  const jobDetailOverflowsAt320 = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(jobDetailOverflowsAt320).toBe(false);
  if (process.env.VISUAL_CAPTURE) {
    await page.setViewportSize({ width: 930, height: 720 });
    await page.screenshot({
      path: "test-results/desktop-drop-detail.png"
    });
  }
  await page.setViewportSize({ width: 1280, height: 720 });
  await jobDetail.getByRole("button", { name: "Set up Drop" }).click();
  await expect(
    page.getByRole("heading", { name: "Set up Drop" })
  ).toBeVisible();
  await page.getByRole("button", { name: "Choose folder" }).click();
  await expect(page.getByText("/Users/max/Downloads/Spare")).toBeVisible();
  await page.getByRole("button", { name: "Start Drop" }).click();
  const transformation = page.locator(".transformation-page");
  await expect(
    transformation.getByRole("heading", { name: "Preparing Drop" })
  ).toBeVisible();
  const viewport = page.viewportSize();
  const transformationContentBox = await transformation
    .locator(".transformation-content")
    .boundingBox();
  const transformationStepsBox = await transformation
    .locator(".transformation-steps")
    .boundingBox();
  if (!viewport || !transformationContentBox || !transformationStepsBox) {
    throw new Error("Unable to measure the transformation layout.");
  }
  expect(
    Math.abs(
      transformationContentBox.x +
        transformationContentBox.width / 2 -
        viewport.width / 2
    )
  ).toBeLessThanOrEqual(1);
  expect(
    Math.abs(
      transformationStepsBox.x +
        transformationStepsBox.width / 2 -
        viewport.width / 2
    )
  ).toBeLessThanOrEqual(1);
  expect(
    Math.abs(
      transformationContentBox.y +
        transformationContentBox.height / 2 -
        viewport.height / 2
    )
  ).toBeLessThanOrEqual(1);
  const finalMoment = page.waitForFunction(() => {
    const transformationHeading = document.querySelector(
      ".transformation-page.is-complete h1"
    );
    const titlebar = document.querySelector(".desktop-titlebar > p");
    const windowState = document.querySelector(".desktop-window-state");
    return (
      transformationHeading?.textContent ===
        "This computer is now a Drop." &&
      titlebar?.textContent === "This computer is now a Drop" &&
      windowState?.textContent?.trim() === "Working"
    );
  });
  await expect(transformation.getByText("Preparing storage")).toBeVisible();
  await expect(transformation.getByText("Checking permissions")).toBeVisible();
  await expect(transformation.getByText("Opening local access")).toBeVisible();
  await expect(transformation.getByText("Starting Drop")).toBeVisible();
  await expect(page.locator(".desktop-titlebar > p")).toHaveText(
    "Starting Drop"
  );
  await expect(page.locator(".desktop-window-state")).toContainText("Starting");
  await expect(
    sidebar.getByRole("button", { name: "Home" })
  ).toBeDisabled();
  const currentMarker = transformation.locator(
    ".transformation-step.is-current .transformation-marker"
  );
  await expect
    .poll(() =>
      currentMarker.evaluate(
        (marker) => getComputedStyle(marker, "::after").animationName
      )
    )
    .toBe("transformation-pulse");
  await expect
    .poll(() =>
      transformation
        .locator(".transformation-steps")
        .evaluate(
          (steps) => getComputedStyle(steps, "::after").transitionDuration
        )
    )
    .toBe("0.9s");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect
    .poll(() =>
      currentMarker.evaluate(
        (marker) => getComputedStyle(marker, "::after").animationName
      )
    )
    .toBe("none");
  await expect(transformation.locator(".transformation-steps")).toHaveCSS(
    "transition-duration",
    "0s"
  );
  await page.emulateMedia({ reducedMotion: "no-preference" });
  const transformationA11y = await new AxeBuilder({ page })
    .include(".transformation-page")
    .analyze();
  expect(transformationA11y.violations).toEqual([]);
  if (process.env.VISUAL_CAPTURE) {
    await page.setViewportSize({ width: 930, height: 509 });
    await page.screenshot({
      path: "test-results/desktop-drop-starting.png"
    });
  }
  await finalMoment;
  await expect(
    page.getByRole("heading", { name: "This computer is a Drop" })
  ).toBeVisible();
  await expect(
    sidebar.getByRole("button", { name: "Home" })
  ).toBeEnabled();
  await expect(page.getByRole("button", { name: "Open Drop" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Open Received files" })
  ).toBeVisible();
  const dropToolbar = page.getByRole("group", { name: "Drop controls" });
  await expect(dropToolbar.getByRole("button")).toHaveCount(7);
  await expect(
    dropToolbar.getByRole("button", { name: "Add files to Drop" })
  ).toBeVisible();
  await expect(
    dropToolbar.getByRole("button", { name: "Configure" })
  ).toBeVisible();
  await expect(
    dropToolbar.getByRole("button", { name: "View activity" })
  ).toBeVisible();
  await expect(page.locator(".desktop-secondary-actions")).toHaveCount(0);
  await expect(page.locator(".recipe-signal")).toBeVisible();
  await expect(page.locator(".recipe-signal svg")).toHaveCount(0);
  await expect(page.locator(".recipe-signal-mark")).toHaveCSS(
    "mask-image",
    'url("http://127.0.0.1:4173/spare-mark.svg")'
  );
  await expect(page.locator(".desktop-root")).toHaveCSS(
    "font-family",
    /Inter Variable/
  );
  await page.getByRole("button", { name: "Add files to Drop" }).click();
  await expect(page.getByText("8", { exact: true })).toBeVisible();
  const scanQRButton = page.getByRole("button", { name: "Scan QR" });
  await scanQRButton.click();
  const qrDialog = page.getByRole("dialog", { name: "Open Drop nearby" });
  await expect(qrDialog).toBeVisible();
  await expect(
    qrDialog.getByRole("button", { name: "Close QR code" })
  ).toBeFocused();
  await expect(
    qrDialog.getByRole("img", { name: /QR code for/ })
  ).toBeVisible();
  await expect(
    qrDialog.getByText("http://192.168.1.24:7340")
  ).toBeVisible();
  const qrA11y = await new AxeBuilder({ page })
    .include(".desktop-share-dialog")
    .analyze();
  expect(qrA11y.violations).toEqual([]);
  if (process.env.VISUAL_CAPTURE) {
    await page.setViewportSize({ width: 930, height: 509 });
    await page.screenshot({
      path: "test-results/desktop-drop-qr.png"
    });
  }
  await page.keyboard.press("Escape");
  await expect(qrDialog).toHaveCount(0);
  await expect(scanQRButton).toBeFocused();
  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/desktop-drop-main.png"
    });
    await page.screenshot({
      path: "test-results/desktop-drop-home.png",
      fullPage: true
    });
  }

  await page.getByRole("button", { name: "Pause" }).click();
  await expect(page.getByRole("button", { name: "Resume" })).toBeVisible();
  await page.getByRole("button", { name: "Resume" }).click();
  await expect(page.getByRole("button", { name: "Pause" })).toBeVisible();
  await page.getByRole("button", { name: "Configure" }).click();
  await expect(
    page.getByRole("heading", { name: "Configure Drop" })
  ).toBeVisible();
  await page.getByRole("button", { name: "Save configuration" }).click();
  await expect(
    page.getByRole("heading", { name: "This computer is a Drop" })
  ).toBeVisible();

  await page
    .getByRole("button", { name: "Activity", exact: true })
    .click();
  await expect(
    page.locator(".desktop-activity-list").getByText("Drop started")
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Show file" })
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Technical details" })
  ).toHaveCount(0);

  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/desktop-drop-activity.png",
      fullPage: true
    });
  }

  await page.setViewportSize({ width: 320, height: 800 });
  const overflowsAt320 = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(overflowsAt320).toBe(false);

  await page.setViewportSize({ width: 640, height: 800 });
  await page.evaluate(() => {
    document.documentElement.style.fontSize = "200%";
  });
  const overflowsAt200Percent = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(overflowsAt200Percent).toBe(false);

  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Appearance" })
  ).toBeVisible();
  const settingsSectionOrder = await page
    .locator(".settings-groups > section > h2")
    .allTextContents();
  expect(settingsSectionOrder.indexOf("Notifications")).toBeLessThan(
    settingsSectionOrder.indexOf("Appearance")
  );
  const desktopRoot = page.locator(".desktop-root");
  const darkTheme = page.getByRole("radio", { name: /^Dark/ });
  const lightTheme = page.getByRole("radio", { name: /^Light/ });
  await expect(page.getByRole("radio")).toHaveCount(2);
  await expect(page.locator(".theme-choice small")).toHaveCount(0);
  await expect(page.getByRole("radio", { name: /^Clear/ })).toHaveCount(0);
  await expect(darkTheme).toBeChecked();
  await lightTheme.check();
  await expect(desktopRoot).toHaveAttribute("data-theme", "light");
  await expect(desktopRoot).toHaveCSS("background-color", "rgb(241, 241, 241)");
  const lightThemeA11y = await new AxeBuilder({ page })
    .include(".desktop-root")
    .analyze();
  expect(lightThemeA11y.violations).toEqual([]);
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(
    page.getByRole("paragraph").filter({ hasText: "Settings saved." })
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Notifications" })
  ).toBeVisible();
  await expect(page.getByText("Drop notifications")).toBeVisible();
  await expect(page.getByText("Site notifications")).toBeVisible();
  await expect(page.getByText("Hook notifications")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Export backup" })
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Open remote dashboard" })
  ).toBeVisible();
  await page.getByRole("button", { name: "Open remote dashboard" }).click();
  await expect(
    page
      .getByRole("paragraph")
      .filter({ hasText: "The remote dashboard opened in your browser." })
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Show selected folder" })
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Choose another folder" })
  ).toBeVisible();
  const settingsOverflowAt200Percent = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(settingsOverflowAt200Percent).toBe(false);
  await page.evaluate(() => {
    document.documentElement.style.fontSize = "100%";
  });
  await page.setViewportSize({ width: 930, height: 509 });
  const settingsLayout = await page.evaluate(() => {
    const main = document.querySelector<HTMLElement>(".desktop-main")!;
    const mainBounds = main.getBoundingClientRect();
    const sections = Array.from(
      document.querySelectorAll<HTMLElement>(".settings-groups > section")
    ).map((section) => section.getBoundingClientRect());
    const heading = getComputedStyle(
      document.querySelector<HTMLElement>(".desktop-page h1")!
    );
    const emphasis = getComputedStyle(
      document.querySelector<HTMLElement>(".setting-row strong")!
    );
    return {
      bodyScrolls: document.body.scrollHeight > document.body.clientHeight,
      mainScrolls: main.scrollHeight > main.clientHeight,
      sectionsFit: sections.every(
        (section) =>
          section.left >= mainBounds.left &&
          section.right <= mainBounds.right
      ),
      headingWeight: heading.fontWeight,
      emphasisWeight: emphasis.fontWeight
    };
  });
  expect(settingsLayout).toEqual({
    bodyScrolls: false,
    mainScrolls: true,
    sectionsFit: true,
    headingWeight: "600",
    emphasisWeight: "500"
  });
  await page.setViewportSize({ width: 320, height: 800 });
  const settingsOverflowAt320 = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(settingsOverflowAt320).toBe(false);
  if (process.env.VISUAL_CAPTURE) {
    await page.setViewportSize({ width: 1120, height: 760 });
    await page.getByRole("heading", { name: "Appearance" }).evaluate((heading) => {
      heading.scrollIntoView({ block: "start" });
      const main = document.querySelector<HTMLElement>(".desktop-main");
      if (main) main.scrollTop = Math.max(0, main.scrollTop - 56);
    });
    await page.screenshot({
      path: "test-results/desktop-settings-light.png",
      fullPage: true
    });
  }
});
