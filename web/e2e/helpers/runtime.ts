import type { Locator, Page } from "@playwright/test";

/** Synchronize on React's applied server snapshot, not only a finished HTTP call. */
export async function waitForRoomRevision(
  page: Page,
  id: string,
  revision: number,
) {
  // Poll in the renderer: a one-second timeout on each CDP DOM query reports
  // false negatives when multiple software-rendered boards occupy the CPU.
  await page.waitForFunction(
    ({ id, revision }) => {
      const app = document.querySelector<HTMLElement>(".app");
      return (
        app?.dataset.roomId === id &&
        Number(app.dataset.roomRevision) >= revision
      );
    },
    { id, revision },
    { timeout: 30000, polling: 100 },
  );
}

/** Explicit stress mode for reproducing slow main-thread updates locally. */
export async function throttleBrowserCPU(page: Page) {
  const rate = Number(process.env.WESTEROS_CPU_THROTTLE || 1);
  if (!Number.isFinite(rate) || rate < 1 || rate > 8)
    throw new Error("WESTEROS_CPU_THROTTLE must be between 1 and 8");
  if (rate === 1) return;
  const session = await page.context().newCDPSession(page);
  await session.send("Emulation.setCPUThrottlingRate", { rate });
}

/** Passing-test pictures are optional; failure screenshots/DOM traces stay on.
 * GPU readback can take minutes on SwiftShader and is not a functional check.
 */
export async function captureVisualCheckpoint(
  target: Page | Locator,
  path: string,
) {
  if (process.env.WESTEROS_VISUAL_CHECKS !== "1") return;
  await target.screenshot({
    path,
    animations: "disabled",
    scale: "css",
    timeout: 15000,
  });
}
