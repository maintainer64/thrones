import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 45000,
  use: {
    baseURL: process.env.BASE_URL || "http://127.0.0.1:18081",
    viewport: { width: 1600, height: 1000 },
    headless: true,
    // Managed CI uses real Chromium's new headless mode, matching local Chrome,
    // not the separate legacy shell whose WebGL context lifecycle differs.
    channel:
      process.env.PLAYWRIGHT_MANAGED_CHROMIUM === "1" ? "chromium" : undefined,
    launchOptions: {
      args: [
        // Multiple independent players must keep receiving WS snapshots in
        // headless Chrome, even when another test tab has foreground focus.
        "--disable-renderer-backgrounding",
        "--disable-background-timer-throttling",
        "--disable-backgrounding-occluded-windows",
        ...(process.env.SOFTWARE_WEBGL === "1"
          ? ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"]
          : []),
      ],
      executablePath:
        process.env.CHROME_EXECUTABLE_PATH ||
        (process.env.PLAYWRIGHT_MANAGED_CHROMIUM === "1"
          ? undefined
          : process.platform === "darwin"
            ? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
            : "/usr/bin/google-chrome"),
    },
    screenshot: "only-on-failure",
    // Continuous GPU screencasts can stall SwiftShader. Keep DOM snapshots,
    // network and sources; on-failure screenshots still capture the final UI.
    trace: {
      mode: "retain-on-failure",
      screenshots: false,
      snapshots: true,
      sources: true,
    },
  },
  outputDir: process.env.PLAYWRIGHT_OUTPUT_DIR || "test-results",
});
