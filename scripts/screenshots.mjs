#!/usr/bin/env node
// Screenshots every gomukit widget preview into docs/assets.
//
//   node scripts/screenshots.mjs              # everything, light + dark
//   node scripts/screenshots.mjs --only table # just the Table stories
//   make screenshots
//
// What it does, end to end:
//
//   1. builds and starts examples/harness on a spare port (the harness serves
//      one route per widget story with baked data, catalog at /stories.json),
//   2. drives a headless Chrome over the DevTools protocol -- no npm
//      dependency, no browser download: Node's built-in WebSocket talks CDP to
//      whatever Chrome is already installed,
//   3. loads every story with a minimal MCP Apps host shim injected before the
//      page's own scripts, so the widget completes the ui/initialize handshake
//      and picks up the theme, locale and time zone a real host would send,
//   4. measures the rendered content, resizes the viewport to it and writes a
//      PNG per story and theme into docs/assets/preview,
//   5. rewrites the handful of images the README embeds from the same shots.
//
// Files whose bytes are unchanged are left alone, so a re-run on an untouched
// tree produces an empty git diff.
//
// Rendering depends on the fonts installed on the machine (widgets ask for
// system-ui), so a run on a different OS may rewrite every file.

import { spawn, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import { parseArgs } from "node:util";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

// --- what gets shot -------------------------------------------------------

// Render width per story, in CSS pixels. Widgets lay themselves out against
// the room they are given (container queries, not viewport), so the width is
// the one real composition choice here: it decides how many columns a
// Descriptions list keeps, whether a Choice moves its description into the
// side panel, and how much of a Table fits without wrapping the toolbar.
// Keyed by story id first, then by story group; DEFAULT_WIDTH otherwise.
const WIDTHS = {
  "datepicker-range": 820,
  // Multi-column forms: at the group's 520 they would document themselves
  // collapsed into the single column every form falls back to.
  "form-layout": 780,
  "form-columns": 860,
  "form-sets": 620,
  "table-long": 900,
  Table: 860,
  CardList: 880,
  Card: 560,
  Descriptions: 720,
  Form: 520,
  Menu: 720,
  Confirm: 560,
  Choice: 660,
  "Date picker": 700,
};
const DEFAULT_WIDTH = 800;

// Stories that ship without a data snapshot on purpose: their point is that
// the data arrives at runtime. Shooting them as-is would document an empty
// widget, so the story's own push-panel payload is delivered as a
// ui/notifications/tool-result before the shot.
//
// The other snapshot-less stories (table-empty, cards-empty, card-empty) are
// left alone: the empty rendering IS what they document.
const PUSH_PAYLOAD = new Set(["confirm-runtime", "choice-runtime", "datepicker-runtime"]);

// Images the README embeds, rebuilt from the catalog shots above.
const README_SHOTS = [
  { file: "table.png", story: "table-default", theme: "light" },
  { file: "table-dark.png", story: "table-default", theme: "dark" },
  { file: "form.png", story: "form-edit", theme: "light" },
  { file: "card.png", story: "card-default", theme: "light" },
  { file: "cardlist.png", story: "cards-default", theme: "light" },
  { file: "confirm.png", story: "confirm-danger", theme: "light" },
];

// The host context the widgets are handed. Mirrors what examples/harness's
// host page sends (see the hostContext() builder in host.html), so a shot
// looks like the harness with the matching theme selected.
const HOST_VARS = {
  light: {
    "--color-background-primary": "#ffffff",
    "--color-background-secondary": "#fafafb",
    "--color-text-primary": "#14161a",
    "--color-text-secondary": "#6a7280",
    "--color-border-primary": "#e4e6eb",
  },
  dark: {
    "--color-background-primary": "#16181d",
    "--color-background-secondary": "#1b1e24",
    "--color-text-primary": "#e8eaef",
    "--color-text-secondary": "#949cab",
    "--color-border-primary": "#272b33",
  },
};

const hostContext = (theme) => ({
  theme,
  displayMode: "inline",
  locale: "en-US",
  timeZone: "UTC",
  platform: "web",
  styles: { variables: HOST_VARS[theme] },
});

// --- cli ------------------------------------------------------------------

const { values: opts } = parseArgs({
  options: {
    out: { type: "string", default: "docs/assets/preview" },
    only: { type: "string" },
    themes: { type: "string", default: "light,dark" },
    scale: { type: "string", default: "2" },
    width: { type: "string" },
    port: { type: "string" },
    url: { type: "string" },
    chrome: { type: "string" },
    "no-readme": { type: "boolean", default: false },
    help: { type: "boolean", default: false },
  },
});

if (opts.help) {
  console.log(`usage: node scripts/screenshots.mjs [options]

  --out DIR      where the catalog PNGs go (default docs/assets/preview)
  --only LIST    comma separated story ids or id prefixes, e.g. table,card-
  --themes LIST  light, dark or both (default light,dark)
  --scale N      device pixel ratio (default 2)
  --width N      force one render width for every story
  --port N       port for the harness this script starts (default: a free one)
  --url URL      shoot an already running harness instead of starting one
  --chrome PATH  Chrome/Chromium binary (default: \$GOMUKIT_CHROME, then PATH)
  --no-readme    skip rewriting the images the README embeds
`);
  process.exit(0);
}

const themes = opts.themes.split(",").map((s) => s.trim()).filter(Boolean);
for (const t of themes) {
  if (t !== "light" && t !== "dark") fail(`unknown theme ${t}`);
}
const scale = Number(opts.scale);
const forcedWidth = opts.width ? Number(opts.width) : 0;
const filters = (opts.only ?? "").split(",").map((s) => s.trim()).filter(Boolean);
const outDir = path.resolve(repoRoot, opts.out);

function fail(msg) {
  console.error(`screenshots: ${msg}`);
  process.exit(1);
}

// --- minimal chrome devtools protocol client ------------------------------

class CDP {
  #ws;
  #next = 1;
  #pending = new Map();
  #listeners = new Set();

  static async connect(url) {
    const ws = new WebSocket(url);
    await new Promise((resolve, reject) => {
      ws.addEventListener("open", resolve, { once: true });
      ws.addEventListener("error", () => reject(new Error(`cannot reach ${url}`)), { once: true });
    });
    return new CDP(ws);
  }

  constructor(ws) {
    this.#ws = ws;
    ws.addEventListener("message", (ev) => {
      const msg = JSON.parse(ev.data);
      if (msg.id !== undefined) {
        const p = this.#pending.get(msg.id);
        if (!p) return;
        this.#pending.delete(msg.id);
        if (msg.error) p.reject(new Error(`${p.method}: ${msg.error.message}`));
        else p.resolve(msg.result);
        return;
      }
      for (const fn of this.#listeners) fn(msg);
    });
  }

  send(method, params = {}, sessionId) {
    const id = this.#next++;
    const msg = { id, method, params };
    if (sessionId) msg.sessionId = sessionId;
    return new Promise((resolve, reject) => {
      this.#pending.set(id, { resolve, reject, method });
      this.#ws.send(JSON.stringify(msg));
    });
  }

  // Resolves on the next matching event. Registered before the command that
  // triggers it, so a fast event cannot be missed.
  once(method, sessionId, timeoutMs = 30_000) {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.#listeners.delete(fn);
        reject(new Error(`timed out waiting for ${method}`));
      }, timeoutMs);
      const fn = (msg) => {
        if (msg.method !== method) return;
        if (sessionId && msg.sessionId !== sessionId) return;
        clearTimeout(timer);
        this.#listeners.delete(fn);
        resolve(msg.params);
      };
      this.#listeners.add(fn);
    });
  }

  close() {
    this.#ws.close();
  }
}

// --- process helpers ------------------------------------------------------

const children = [];

function run(cmd, args, o = {}) {
  const child = spawn(cmd, args, { cwd: repoRoot, ...o });
  children.push(child);
  return child;
}

function stopAll() {
  const stopping = [];
  for (const child of children) {
    if (child.exitCode !== null || child.signalCode !== null) continue;
    child.kill("SIGTERM");
    stopping.push(
      Promise.race([
        new Promise((r) => child.on("exit", r)),
        sleep(3000).then(() => child.kill("SIGKILL")),
      ]),
    );
  }
  return Promise.all(stopping);
}

async function exec(cmd, args) {
  const child = run(cmd, args, { stdio: ["ignore", "inherit", "inherit"] });
  const code = await new Promise((resolve) => child.on("exit", resolve));
  if (code !== 0) fail(`${cmd} ${args.join(" ")} exited ${code}`);
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function waitForHTTP(url, tries = 100) {
  for (let i = 0; i < tries; i++) {
    try {
      const res = await fetch(url, { signal: AbortSignal.timeout(1000) });
      if (res.ok) return res;
    } catch {
      // not up yet
    }
    await sleep(100);
  }
  fail(`nothing answering at ${url}`);
}

// --- harness --------------------------------------------------------------

// Asks the kernel for a port nobody is on, so a run never collides with
// whatever else the machine is serving.
function freePort() {
  return new Promise((resolve, reject) => {
    const srv = createServer();
    srv.on("error", reject);
    srv.listen(0, "127.0.0.1", () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
  });
}

// Builds the harness and starts it. `go run` is deliberately avoided: it
// leaves the server running when the wrapper is killed.
async function startHarness(workDir, port) {
  const bin = path.join(workDir, "harness");
  console.log("building examples/harness...");
  await exec("go", ["build", "-o", bin, "./examples/harness"]);
  const child = run(bin, ["-addr", `127.0.0.1:${port}`], { stdio: ["ignore", "ignore", "pipe"] });
  let stderr = "";
  child.stderr.on("data", (c) => (stderr += c));
  child.on("exit", (code) => {
    if (code !== 0 && code !== null) fail(`harness exited ${code}:\n${stderr.trim()}`);
  });
  const base = `http://127.0.0.1:${port}`;
  await waitForHTTP(`${base}/stories.json`);
  return base;
}

// --- chrome ---------------------------------------------------------------

function chromePath() {
  const explicit = opts.chrome ?? process.env.GOMUKIT_CHROME;
  if (explicit) return explicit;
  for (const c of ["google-chrome", "google-chrome-stable", "chromium", "chromium-browser"]) {
    const { status, stdout } = spawnSync("which", [c], { encoding: "utf8" });
    if (status === 0 && stdout.trim() !== "") return stdout.trim();
  }
  fail("no Chrome found; install one or pass --chrome /path/to/chrome");
}

async function startChrome(workDir) {
  const bin = chromePath();
  const child = run(bin, [
    "--headless=new",
    "--remote-debugging-port=0",
    "--remote-allow-origins=*",
    `--user-data-dir=${path.join(workDir, "chrome")}`,
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-extensions",
    "--disable-background-networking",
    "--disable-gpu",
    "--hide-scrollbars",
    "--force-color-profile=srgb",
    "--font-render-hinting=none",
    "about:blank",
  ], { stdio: ["ignore", "ignore", "pipe"] });

  // Chrome prints "DevTools listening on ws://..." on stderr once the
  // debugging endpoint is up. --remote-debugging-port=0 picks a free port, so
  // this line is the only way to learn it.
  const endpoint = await new Promise((resolve, reject) => {
    let buf = "";
    const timer = setTimeout(() => reject(new Error("chrome did not report a devtools endpoint")), 30_000);
    child.stderr.on("data", (chunk) => {
      buf += chunk;
      const m = buf.match(/ws:\/\/\S+/);
      if (m) {
        clearTimeout(timer);
        child.stderr.removeAllListeners("data");
        child.stderr.resume();
        resolve(m[0]);
      }
    });
    child.on("exit", (code) => reject(new Error(`chrome exited ${code}`)));
  });

  return CDP.connect(endpoint);
}

// --- the host shim injected into every story ------------------------------

// Runs before the widget's own scripts, so the ui/initialize request the
// runtime fires on boot finds an answer. A story page loaded at the top level
// posts to window.parent === window, so the shim just listens on the same
// window and replies there.
const hostShim = (theme) => `(() => {
  const hostContext = ${JSON.stringify(hostContext(theme))};
  addEventListener("message", (ev) => {
    const msg = ev.data;
    if (!msg || msg.jsonrpc !== "2.0" || msg.id === undefined || !msg.method) return;
    const result = msg.method === "ui/initialize"
      ? { protocolVersion: "2026-01-26", hostContext }
      : {};
    window.postMessage({ jsonrpc: "2.0", id: msg.id, result }, "*");
  });
  // Used for the stories that only get their data at runtime.
  window.__gomukitPush = (structuredContent) => {
    window.postMessage({
      jsonrpc: "2.0",
      method: "ui/notifications/tool-result",
      params: {
        structuredContent,
        content: [{ type: "text", text: JSON.stringify(structuredContent) }],
      },
    }, "*");
  };
})();`;

// --- shooting -------------------------------------------------------------

async function newPage(cdp) {
  const { targetId } = await cdp.send("Target.createTarget", { url: "about:blank" });
  const { sessionId } = await cdp.send("Target.attachToTarget", { targetId, flatten: true });
  await cdp.send("Page.enable", {}, sessionId);
  await cdp.send("Runtime.enable", {}, sessionId);
  // The canvas is deliberately unpainted by the widget CSS (base.css), so
  // without this Chrome fills it white -- a white ring around every dark shot.
  await cdp.send("Emulation.setDefaultBackgroundColorOverride", { color: { r: 0, g: 0, b: 0, a: 0 } }, sessionId);
  return sessionId;
}

async function evaluate(cdp, sessionId, expression) {
  const { result, exceptionDetails } = await cdp.send(
    "Runtime.evaluate",
    { expression, awaitPromise: true, returnByValue: true },
    sessionId,
  );
  if (exceptionDetails) throw new Error(exceptionDetails.text ?? "evaluate failed");
  return result.value;
}

async function shoot(cdp, sessionId, { url, theme, width, payload }) {
  // prefers-color-scheme is pinned as well as the host theme: a widget that
  // never gets a host context falls back to the media query, and the shots
  // should not depend on the machine's desktop theme.
  await cdp.send("Emulation.setEmulatedMedia", {
    features: [{ name: "prefers-color-scheme", value: theme }],
  }, sessionId);
  await cdp.send("Emulation.setDeviceMetricsOverride", {
    width, height: 900, deviceScaleFactor: scale, mobile: false,
  }, sessionId);

  const shim = await cdp.send("Page.addScriptToEvaluateOnNewDocument", { source: hostShim(theme) }, sessionId);
  try {
    const loaded = cdp.once("Page.loadEventFired", sessionId);
    await cdp.send("Page.navigate", { url }, sessionId);
    await loaded;

    if (payload) {
      await evaluate(cdp, sessionId, `window.__gomukitPush(${payload})`);
    }

    // Let fonts, the handshake re-render (host locale/timeZone) and any
    // container-query reflow settle before measuring.
    await evaluate(cdp, sessionId, "document.fonts.ready");
    await settle(cdp, sessionId);

    // Measure the body, not the document: the document element stretches to
    // the viewport, while the body is auto-height around the widget plus the
    // page gutter -- exactly the crop an image wants.
    const height = await evaluate(
      cdp,
      sessionId,
      "Math.max(1, Math.ceil(Math.max(document.body.getBoundingClientRect().height, document.body.scrollHeight)))",
    );
    await cdp.send("Emulation.setDeviceMetricsOverride", {
      width, height, deviceScaleFactor: scale, mobile: false,
    }, sessionId);
    await settle(cdp, sessionId);

    const { data } = await cdp.send("Page.captureScreenshot", {
      format: "png", captureBeyondViewport: true, fromSurface: true,
    }, sessionId);
    return { png: Buffer.from(data, "base64"), width, height };
  } finally {
    await cdp.send("Page.removeScriptToEvaluateOnNewDocument", { identifier: shim.identifier }, sessionId);
  }
}

const settle = (cdp, sessionId) =>
  evaluate(
    cdp,
    sessionId,
    "new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 120))))",
  );

// Writes only when the bytes differ, so an unchanged run leaves git clean.
async function writeIfChanged(file, buf) {
  try {
    const old = await readFile(file);
    if (old.equals(buf)) return false;
  } catch {
    // new file
  }
  await writeFile(file, buf);
  return true;
}

const digest = (buf) => createHash("sha256").update(buf).digest("hex").slice(0, 8);

// --- main -----------------------------------------------------------------

async function main() {
  const workDir = await mkdtemp(path.join(tmpdir(), "gomukit-shots-"));
  let cdp;
  try {
    const base = opts.url ?? (await startHarness(workDir, opts.port ? Number(opts.port) : await freePort()));
    const stories = await (await waitForHTTP(`${base}/stories.json`)).json();

    const wanted = stories.filter(
      (s) => filters.length === 0 || filters.some((f) => s.id === f || s.id.startsWith(f) || s.group.toLowerCase().startsWith(f.toLowerCase())),
    );
    if (wanted.length === 0) fail(`--only ${opts.only} matched no stories`);

    await mkdir(outDir, { recursive: true });
    cdp = await startChrome(workDir);
    const sessionId = await newPage(cdp);

    const shots = new Map(); // "<story>:<theme>" -> png
    let written = 0;

    for (const story of wanted) {
      for (const theme of themes) {
        const width = forcedWidth || WIDTHS[story.id] || WIDTHS[story.group] || DEFAULT_WIDTH;
        // ?transparent=0 is the framed variant: the widget sits on its own
        // page fill instead of a transparent canvas, which is what a
        // standalone image wants.
        const url = `${base}/story/${story.id}?transparent=0`;
        const payload = PUSH_PAYLOAD.has(story.id) ? story.payload : null;
        const shot = await shoot(cdp, sessionId, { url, theme, width, payload });

        shots.set(`${story.id}:${theme}`, shot.png);
        const name = theme === "dark" ? `${story.id}-dark.png` : `${story.id}.png`;
        const changed = await writeIfChanged(path.join(outDir, name), shot.png);
        if (changed) written++;
        console.log(
          `${changed ? "wrote  " : "same   "} ${path.relative(repoRoot, path.join(outDir, name))}` +
            `  ${shot.width}x${shot.height} @${scale}x  ${digest(shot.png)}`,
        );
      }
    }

    if (!opts["no-readme"]) {
      const assets = path.resolve(repoRoot, "docs/assets");
      await mkdir(assets, { recursive: true });
      for (const shot of README_SHOTS) {
        const png = shots.get(`${shot.story}:${shot.theme}`);
        if (!png) {
          console.log(`skip    docs/assets/${shot.file} (${shot.story}/${shot.theme} not in this run)`);
          continue;
        }
        const changed = await writeIfChanged(path.join(assets, shot.file), png);
        if (changed) written++;
        console.log(`${changed ? "wrote  " : "same   "} docs/assets/${shot.file}  <- ${shot.story} (${shot.theme})`);
      }
    }

    console.log(`\n${wanted.length} stories x ${themes.length} theme(s), ${written} file(s) changed`);
  } finally {
    cdp?.close();
    // Chrome unlinks its profile as it goes down, so wait for it before
    // clearing the work directory out from under it.
    await stopAll();
    await rm(workDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  }
}

process.on("SIGINT", () => {
  stopAll();
  process.exit(130);
});

await main();
