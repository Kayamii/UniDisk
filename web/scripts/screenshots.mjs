// Captures the README screenshots from the demo build.
//
//   npm run build:demo && npm run screenshots
//
// Boots `vite preview` against dist/, drives each screen with Playwright, and
// writes PNGs to ../docs/screenshots/. Re-run it any time the UI changes.

import { spawn } from "node:child_process";
import { mkdir, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { chromium } from "@playwright/test";

const here = path.dirname(fileURLToPath(import.meta.url));
const webDir = path.resolve(here, "..");
const outDir = path.resolve(webDir, "../docs/screenshots");

const PORT = 4174;
const BASE = `http://localhost:${PORT}/UniDisk/`;
const VIEWPORT = { width: 1440, height: 900 };

/**
 * startPreview runs `vite preview` and resolves once it's serving. It calls
 * vite's JS entry through node rather than the npx shim, which spawn() cannot
 * launch directly on Windows.
 */
function startPreview() {
  const viteBin = path.resolve(webDir, "node_modules/vite/bin/vite.js");
  const child = spawn(
    process.execPath,
    [viteBin, "preview", "--port", String(PORT), "--strictPort"],
    {
      cwd: webDir,
      stdio: ["ignore", "inherit", "inherit"],
      // preview resolves `base` from the same config as the build, so it must
      // see VITE_DEMO too or it will serve at / while the assets expect /UniDisk/.
      env: { ...process.env, VITE_DEMO: "1" },
    }
  );
  return waitForServer().then(() => child, (err) => {
    child.kill();
    throw err;
  });
}

/** waitForServer polls the preview URL until it answers (or gives up). */
async function waitForServer() {
  const deadline = Date.now() + 30_000;
  for (;;) {
    try {
      const res = await fetch(BASE);
      if (res.ok) return;
    } catch {
      // not listening yet
    }
    if (Date.now() > deadline) throw new Error("preview server did not start");
    await new Promise((r) => setTimeout(r, 300));
  }
}

/**
 * shot captures to docs/screenshots/<name>.png. Toasts from earlier steps are
 * dismissed first so they don't linger in an unrelated screenshot. Pass
 * fullPage for screens that are taller than the viewport.
 */
async function shot(page, name, { fullPage = false } = {}) {
  await dismissToasts(page);
  await page.waitForTimeout(450); // let animations and skeletons settle
  await page.screenshot({ path: path.join(outDir, `${name}.png`), fullPage });
  console.log(`  ✓ ${name}.png`);
}

/**
 * dismissToasts waits for any visible sonner toast to auto-expire, so a toast
 * raised by an earlier step doesn't bleed into a later screenshot.
 */
async function dismissToasts(page) {
  const toasts = page.locator("[data-sonner-toast]");
  if ((await toasts.count()) === 0) return;
  await toasts.first().waitFor({ state: "detached", timeout: 8000 }).catch(() => {});
}

/** signIn goes to a fresh demo session and logs in as the seeded admin. */
async function signIn(page) {
  await page.goto(BASE, { waitUntil: "networkidle" });
  await page.evaluate(() => localStorage.clear());
  await page.goto(BASE, { waitUntil: "networkidle" });
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByText("Storage Pool").waitFor();
}

async function main() {
  await rm(outDir, { recursive: true, force: true });
  await mkdir(outDir, { recursive: true });

  const preview = await startPreview();
  const browser = await chromium.launch();
  const page = await browser.newPage({
    viewport: VIEWPORT,
    deviceScaleFactor: 2, // crisp on high-DPI screens / GitHub's rendering
  });

  try {
    // 1. Login screen (dark, as shipped)
    await page.goto(BASE, { waitUntil: "networkidle" });
    await page.evaluate(() => localStorage.clear());
    await page.reload({ waitUntil: "networkidle" });
    await shot(page, "login");

    await signIn(page);

    // 2. Files — list view, the default landing screen
    await shot(page, "files");

    // 3. Files — grid view
    await page.locator("button.rounded-l-none").first().click();
    await shot(page, "files-grid");
    await page.locator("button.rounded-r-none").first().click();

    // 4. In-app preview of a text file
    await page.getByRole("button", { name: /README\.md/ }).click();
    await page.waitForTimeout(600);
    await shot(page, "preview");
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);

    // 5. Share link dialog, with a link already created
    const row = page.locator("tr", { hasText: "budget-2026.csv" });
    await row.locator("button").last().click();
    await page.getByRole("menuitem", { name: "Share link" }).click();
    await page.getByRole("button", { name: "Create link" }).click();
    await page.waitForTimeout(500);
    await shot(page, "share-link");
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);

    // 6. Storage pool — the aggregation story in one screen. Captured tall so
    // every connected account is visible.
    await page.getByRole("link", { name: "Storage Pool" }).click();
    await page.getByText("Total capacity").waitFor();
    await page.setViewportSize({ width: VIEWPORT.width, height: 1250 });
    await shot(page, "pool");
    await page.setViewportSize(VIEWPORT);

    // 7. Providers — connected accounts
    await page.getByRole("link", { name: "Providers" }).click();
    await shot(page, "providers");

    // 8. Add provider — the schema-driven credential form
    await page.getByRole("button", { name: "Add provider" }).click();
    await page.getByRole("button", { name: /S3-compatible/ }).click();
    await page.waitForTimeout(300);
    await shot(page, "add-provider");
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);

    // 9. Roles — RBAC privilege matrix
    await page.getByRole("link", { name: "Roles" }).click();
    await shot(page, "roles");

    // 10. Users
    await page.getByRole("link", { name: "Users" }).click();
    await shot(page, "users");

    // 11. API keys
    await page.getByRole("link", { name: "API Keys" }).click();
    await shot(page, "api-keys");

    // 12. Light theme, on the files screen
    await page.getByRole("link", { name: "Files", exact: true }).click();
    await page.getByTitle("Toggle theme").click();
    await shot(page, "files-light");
  } finally {
    await browser.close();
    preview.kill();
  }

  console.log(`\nScreenshots written to ${outDir}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
