import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import {
  captureVisualCheckpoint,
  clickOrWaitForCompletion,
  throttleBrowserCPU,
} from "../../web/e2e/helpers/runtime.ts";
const require = createRequire(
  new URL("../../web/package.json", import.meta.url),
);
const { errors } = require("@playwright/test");

test("a shortcut waits for its click to settle and then verifies completion", async () => {
  const steps = [];
  await clickOrWaitForCompletion(
    {
      click: async (options) => {
        assert.equal(options.timeout, 3000);
        steps.push("click settled");
      },
    },
    async () => steps.push("completed outcome checked"),
  );
  assert.deepEqual(steps, ["click settled", "completed outcome checked"]);
});
test("natural completion during actionability does not wait for a disabled shortcut", async () => {
  let complete = false;
  await clickOrWaitForCompletion(
    {
      click: async () => {
        complete = true;
        throw new errors.TimeoutError("animation disabled the button");
      },
    },
    async () => assert.equal(complete, true),
  );
});
test("a shortcut timeout cannot hide an unfinished animation", async () => {
  const unfinished = new Error("animation is still playing");
  await assert.rejects(
    clickOrWaitForCompletion(
      {
        click: async () => {
          throw new errors.TimeoutError("not actionable");
        },
      },
      async () => {
        throw unfinished;
      },
    ),
    (error) => error === unfinished,
  );
});
test("a successful shortcut still must produce the expected outcome", async () => {
  const unfinished = new Error("wrong final army");
  await assert.rejects(
    clickOrWaitForCompletion({ click: async () => undefined }, async () => {
      throw unfinished;
    }),
    (error) => error === unfinished,
  );
});
test("unexpected action errors are preserved even if completion would pass", async () => {
  const disconnected = new Error("browser disconnected");
  let checked = false;
  await assert.rejects(
    clickOrWaitForCompletion(
      {
        click: async () => {
          throw disconnected;
        },
      },
      async () => {
        checked = true;
      },
    ),
    (error) => error === disconnected,
  );
  assert.equal(checked, false);
});

function environment(t, key, value) {
  const previous = process.env[key];
  process.env[key] = value;
  t.after(() => {
    if (previous === undefined) delete process.env[key];
    else process.env[key] = previous;
  });
}

test("passing-test pictures require explicit opt-in", async (t) => {
  environment(t, "WESTEROS_VISUAL_CHECKS", "");
  const screenshot = t.mock.fn();
  await captureVisualCheckpoint({ screenshot }, "checkpoint.png");
  assert.equal(screenshot.mock.calls.length, 0);
});
test("requested pictures have bounded, animation-free CSS-scale capture", async (t) => {
  environment(t, "WESTEROS_VISUAL_CHECKS", "1");
  const screenshot = t.mock.fn(async () => undefined);
  await captureVisualCheckpoint({ screenshot }, "checkpoint.png");
  assert.deepEqual(screenshot.mock.calls[0].arguments, [
    {
      path: "checkpoint.png",
      animations: "disabled",
      scale: "css",
      timeout: 15000,
    },
  ]);
});
test("ordinary browser CPU settings remain unchanged", async (t) => {
  environment(t, "WESTEROS_CPU_THROTTLE", "1");
  const context = t.mock.fn();
  await throttleBrowserCPU({ context });
  assert.equal(context.mock.calls.length, 0);
});
test("slow-CPU reproduction uses native CDP", async (t) => {
  environment(t, "WESTEROS_CPU_THROTTLE", "4");
  const send = t.mock.fn(async () => ({}));
  await throttleBrowserCPU({
    context: () => ({ newCDPSession: async () => ({ send }) }),
  });
  assert.deepEqual(send.mock.calls[0].arguments, [
    "Emulation.setCPUThrottlingRate",
    { rate: 4 },
  ]);
});
test("invalid stress settings are rejected", async (t) => {
  environment(t, "WESTEROS_CPU_THROTTLE", "");
  for (const rate of ["0", "9", "not-a-number"]) {
    process.env.WESTEROS_CPU_THROTTLE = rate;
    await assert.rejects(throttleBrowserCPU({}), /between 1 and 8/);
  }
});
