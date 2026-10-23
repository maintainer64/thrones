import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { createRequire } from "node:module";

const require = createRequire(
  new URL("../../web/package.json", import.meta.url),
);
const { parse } = require("yaml");
const root = new URL("../../", import.meta.url);
const workflow = parse(
  await readFile(new URL(".github/workflows/ci.yml", root), "utf8"),
);

test("CI runs every Go browser suite in its own bounded matrix job", async () => {
  const discovered = [];
  for (const name of await readdir(new URL("internal/server/", root))) {
    if (!name.endsWith("_browser_test.go")) continue;
    const source = await readFile(
      new URL(`internal/server/${name}`, root),
      "utf8",
    );
    for (const match of source.matchAll(
      /^func Test(\w+)Browser\(t \*testing\.T\)/gm,
    ))
      discovered.push(match[1]);
  }
  const browser = workflow.jobs.browser;
  assert.deepEqual(
    [...browser.strategy.matrix.suite].sort(),
    discovered.sort(),
  );
  assert.equal(browser.strategy["fail-fast"], false);
  assert.equal(browser.strategy["max-parallel"], 3);
  const step = browser.steps.find((step) => step.env?.BROWSER_SUITE);
  assert.ok(step.run.includes('"^Test${BROWSER_SUITE}Browser$"'));
  assert.ok(step.run.includes("-timeout=20m"));
  assert.equal(browser["timeout-minutes"], 25);
  assert.ok(
    !workflow.jobs.test.steps.some((step) => step.run?.includes("Browser$")),
  );
});

test("browser jobs reuse matching build/artwork and retain failed diagnostics", () => {
  const browser = workflow.jobs.browser;
  assert.equal(browser.needs, "test");
  const upload = workflow.jobs.test.steps.find(
    (step) => step.with?.name === "browser-build",
  );
  const download = browser.steps.find(
    (step) => step.with?.name === "browser-build",
  );
  assert.ok(upload.with.path.includes("web/dist/"));
  assert.ok(upload.with.path.includes("web/src/artwork/index.json"));
  assert.ok(upload.with.path.includes("local-assets/**/*.webp"));
  assert.ok(upload.with.path.includes("local-assets/*.webp"));
  assert.equal(download.with.path, ".");
  const diagnostics = browser.steps.find((step) =>
    step.with?.name?.startsWith("browser-failure-"),
  );
  assert.equal(diagnostics.if, "failure() || cancelled()");
  assert.ok(diagnostics.with.path.includes("web/test-results/"));
});
