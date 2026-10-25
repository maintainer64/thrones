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
const publication = parse(
  await readFile(new URL(".github/workflows/publish.yml", root), "utf8"),
);

test("only pull requests run tests; releases and ordinary pushes cannot start CI", () => {
  assert.deepEqual(Object.keys(workflow.on), ["pull_request"]);
  assert.equal(workflow.jobs.container, undefined);
  assert.ok(
    !Object.values(workflow.jobs).some((job) =>
      job.steps?.some((step) => step.uses?.startsWith("docker/")),
    ),
  );
});

test("publication is restricted to PRs and published releases without test dependencies", () => {
  assert.deepEqual(Object.keys(publication.on).sort(), [
    "pull_request",
    "release",
  ]);
  assert.deepEqual(publication.on.pull_request.types, [
    "opened",
    "synchronize",
    "reopened",
  ]);
  assert.deepEqual(publication.on.release.types, ["published"]);
  assert.deepEqual(Object.keys(publication.jobs), ["publish"]);
  const job = publication.jobs.publish;
  assert.equal(job.needs, undefined);
  assert.ok(job.if.includes("github.event_name == 'release'"));
  assert.ok(
    job.if.includes(
      "github.event.pull_request.head.repo.full_name == github.repository",
    ),
  );
  assert.equal(publication.permissions.packages, undefined);
  assert.equal(job.permissions.packages, "write");
  assert.ok(
    !job.steps.some((step) => /\b(?:test|test:e2e)\b/.test(step.run || "")),
  );
  const build = job.steps.find((step) =>
    step.uses?.startsWith("docker/build-push-action@"),
  );
  assert.equal(build.with.push, true);
  assert.equal(build.with.platforms, "linux/amd64,linux/arm64");
});

test("PR images use dev-number; latest is restricted to stable SemVer releases", () => {
  const job = publication.jobs.publish;
  const metadata = job.steps.find((step) =>
    step.uses?.startsWith("docker/metadata-action@"),
  );
  assert.equal(metadata.with.images, "ghcr.io/${{ github.repository }}");
  assert.deepEqual(metadata.with.tags.trim().split("\n"), [
    "type=ref,event=pr,prefix=dev-",
    "type=semver,pattern={{version}},value=${{ github.event.release.tag_name }},enable=${{ github.event_name == 'release' }}",
    "type=semver,pattern={{raw}},value=${{ github.event.release.tag_name }},enable=${{ github.event_name == 'release' }}",
  ]);
  assert.equal(
    metadata.with.flavor,
    "latest=${{ github.event_name == 'release' && !github.event.release.prerelease && 'auto' || 'false' }}",
  );
  const build = job.steps.find((step) =>
    step.uses?.startsWith("docker/build-push-action@"),
  );
  assert.equal(build.with.tags, "${{ steps.meta.outputs.tags }}");
  const login = job.steps.find((step) =>
    step.uses?.startsWith("docker/login-action@"),
  );
  assert.equal(login.with.registry, "ghcr.io");
  assert.equal(login.with.password, "${{ secrets.GITHUB_TOKEN }}");
});

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
