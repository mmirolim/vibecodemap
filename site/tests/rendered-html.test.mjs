import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

async function render() {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);

  return worker.fetch(
    new Request("http://localhost/", { headers: { accept: "text/html" } }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("server-renders the VibeCodeMap product page", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /<title>VibeCodeMap/);
  assert.match(html, /See the codebase/);
  assert.match(html, /Built with Codex \+ GPT-5\.6/);
  assert.match(html, /src="\/demo\.html"/);
  assert.match(html, /github\.com\/mmirolim\/vibecodemap/);
  assert.doesNotMatch(html, /codex-preview|SkeletonPreview|Your site is taking shape/);
});

test("ships the standalone self-map and removes starter dependencies", async () => {
  const [demo, view, packageJson, page] = await Promise.all([
    readFile(new URL("../public/demo.html", import.meta.url), "utf8"),
    readFile(new URL("../public/demo.view.json", import.meta.url), "utf8"),
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    readFile(new URL("../app/page.tsx", import.meta.url), "utf8"),
  ]);

  assert.match(demo, /VibeCodeMap/);
  assert.match(view, /vibecodemap\.view\/0\.1/);
  assert.match(page, /iframe/);
  assert.doesNotMatch(packageJson, /react-loading-skeleton/);
  await assert.rejects(access(new URL("../app/_sites-preview", import.meta.url)));
});
