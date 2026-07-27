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
    page.getByRole("heading", { name: "studio-mac is a Site" })
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
    page.getByRole("heading", { name: "studio-mac is a Drop" })
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
    page.getByRole("heading", { name: "studio-mac is a Hook" })
  ).toBeVisible();
  await expect(page.getByText("Requests received")).toBeVisible();
  await expect(page.getByText("4", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Hook on studio-mac is ready to receive")
  ).toBeVisible();
  await page.getByText("Show instance details").click();
  await expect(page.getByText("Folder", { exact: true })).toHaveCount(0);
});

test("empty state points to all first recipe actions", async ({ page }) => {
  await mockDashboard(page, [], []);
  await page.goto("/");

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
    page.getByRole("heading", { name: "studio-mac is a Site" })
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /Open Site/ })).toBeVisible();
  const hasHorizontalOverflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(hasHorizontalOverflow).toBe(false);
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
  await expect(
    page.getByRole("heading", { name: "Give this computer a job." })
  ).toBeVisible();
  await page.getByRole("button", { name: "Try Drop" }).click();
  await expect(
    page.getByRole("heading", { name: "Set up Drop" })
  ).toBeVisible();
  await page.getByRole("button", { name: "Choose folder" }).click();
  await expect(page.getByText("/Users/max/Downloads/Spare")).toBeVisible();
  await page.getByRole("button", { name: "Start Drop" }).click();
  await expect(
    page.getByRole("heading", { name: "This computer is a Drop" })
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Open Drop" })).toBeVisible();
  await page.getByRole("button", { name: "Add files to Drop" }).click();
  await expect(page.getByText("8 files")).toBeVisible();
  await page.getByRole("button", { name: "Share access" }).click();
  await expect(page.getByRole("img", { name: /QR code for/ })).toBeVisible();
  await expect(page.getByText("http://192.168.1.24:7340")).toBeVisible();
  if (process.env.VISUAL_CAPTURE) {
    await page.screenshot({
      path: "test-results/desktop-drop-home.png",
      fullPage: true
    });
  }

  await page.getByRole("button", { name: "Stop Drop" }).click();
  await expect(page.getByRole("button", { name: "Start Drop" })).toBeVisible();
  await page.getByRole("button", { name: "Start Drop" }).click();
  await expect(page.getByRole("button", { name: "Stop Drop" })).toBeVisible();
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
    page.locator(".desktop-activity-list").getByText("Drop started.")
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Show file" })
  ).toBeVisible();

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
  await page.setViewportSize({ width: 320, height: 800 });
  const settingsOverflowAt320 = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  );
  expect(settingsOverflowAt320).toBe(false);
  if (process.env.VISUAL_CAPTURE) {
    await page.setViewportSize({ width: 1120, height: 760 });
    await page.screenshot({
      path: "test-results/desktop-settings.png",
      fullPage: true
    });
  }
});
