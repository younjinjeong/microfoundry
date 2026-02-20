// Playwright script: login as myadmin, take all screenshots + walkthrough video
import { chromium } from 'playwright';
import { existsSync, mkdirSync } from 'fs';

const BASE = 'https://admin.cf-local.dev:8443';
const IMG_DIR = 'docs/images';
const VIEWPORT = { width: 1440, height: 900 };

// All pages to screenshot (name -> path)
const PAGES = [
  ['dashboard',          '/'],
  ['apps-list',          '/apps'],
  ['services',           '/services'],
  ['catalog',            '/catalog'],
  ['secrets',            '/secrets'],
  ['clusters',           '/clusters'],
  ['monitoring',         '/monitoring'],
  ['users-iam',          '/users'],
  ['workspaces',         '/workspaces'],
  ['docs',               '/docs'],
  ['config',             '/config'],
  ['settings-endpoints', '/settings/endpoints'],
  ['settings-registry',  '/settings/registry'],
  ['settings-webhooks',  '/settings/webhooks'],
  ['settings-smtp',      '/settings/smtp'],
];

// Walkthrough navigation order for video recording
const WALKTHROUGH = [
  { path: '/',                   wait: 2500, label: 'Dashboard' },
  { path: '/apps',               wait: 2000, label: 'Apps' },
  { path: '/services',           wait: 2000, label: 'Services' },
  { path: '/catalog',            wait: 2000, label: 'Catalog' },
  { path: '/secrets',            wait: 2000, label: 'Secrets' },
  { path: '/clusters',           wait: 2000, label: 'Clusters' },
  { path: '/monitoring',         wait: 2000, label: 'Monitoring' },
  { path: '/users',              wait: 2000, label: 'Users & IAM' },
  { path: '/workspaces',         wait: 2000, label: 'Workspaces' },
  { path: '/docs',               wait: 2000, label: 'Documentation' },
  { path: '/config',             wait: 2000, label: 'Configuration' },
  { path: '/settings/endpoints', wait: 2000, label: 'Settings: Endpoints' },
  { path: '/settings/registry',  wait: 2000, label: 'Settings: Registry' },
  { path: '/settings/webhooks',  wait: 2000, label: 'Settings: Webhooks' },
  { path: '/settings/smtp',      wait: 2000, label: 'Settings: SMTP' },
  { path: '/',                   wait: 2000, label: 'Back to Dashboard' },
];

async function login(page) {
  console.log('[login] Navigating to login page...');
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });

  // Click "Sign in with Keycloak"
  console.log('[login] Clicking Sign in with Keycloak...');
  await page.click('a[href="/auth/login"]');

  // Wait for Keycloak login form
  console.log('[login] Waiting for Keycloak form...');
  await page.waitForSelector('#username', { timeout: 15000 });

  // Fill credentials
  await page.fill('#username', 'myadmin');
  await page.fill('#password', 'myadmin');

  // Submit
  console.log('[login] Submitting credentials...');
  await page.click('#kc-login');

  // Wait for redirect back to admin dashboard
  console.log('[login] Waiting for dashboard redirect...');
  await page.waitForURL(`${BASE}/**`, { timeout: 15000 });
  await page.waitForLoadState('networkidle');

  // Verify we're logged in - check for user name in nav or absence of "Sign In" button
  const content = await page.content();
  if (content.includes('myadmin') || !content.includes('Sign in with Keycloak')) {
    console.log('[login] Successfully logged in as myadmin');
  } else {
    console.warn('[login] Warning: login may not have succeeded');
  }
}

async function takeScreenshots(page) {
  console.log('\n=== Taking Screenshots ===\n');

  for (const [name, path] of PAGES) {
    console.log(`[screenshot] ${name} -> ${path}`);
    await page.goto(`${BASE}${path}`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1000); // extra settle time for HTMX
    await page.screenshot({
      path: `${IMG_DIR}/${name}.png`,
      fullPage: false,
    });
    console.log(`  -> saved ${name}.png`);
  }
}

async function recordWalkthrough(context) {
  console.log('\n=== Recording Walkthrough Video ===\n');

  // Create a new page with video recording
  const videoPage = await context.newPage();

  // Login on this page too
  await login(videoPage);

  for (const step of WALKTHROUGH) {
    console.log(`[walkthrough] ${step.label} -> ${step.path}`);
    await videoPage.goto(`${BASE}${step.path}`, { waitUntil: 'networkidle' });
    await videoPage.waitForTimeout(step.wait);
  }

  // Close to finalize video
  await videoPage.close();
  console.log('[walkthrough] Video recording complete');
}

async function main() {
  if (!existsSync(IMG_DIR)) mkdirSync(IMG_DIR, { recursive: true });

  const browser = await chromium.launch({
    headless: true,
    args: ['--ignore-certificate-errors'],
  });

  // Context for screenshots (no video)
  const screenshotCtx = await browser.newContext({
    viewport: VIEWPORT,
    ignoreHTTPSErrors: true,
  });

  const page = await screenshotCtx.newPage();

  // Login and take screenshots
  await login(page);
  await takeScreenshots(page);
  await screenshotCtx.close();

  // Context for walkthrough video
  const videoCtx = await browser.newContext({
    viewport: VIEWPORT,
    ignoreHTTPSErrors: true,
    recordVideo: {
      dir: IMG_DIR,
      size: VIEWPORT,
    },
  });

  await recordWalkthrough(videoCtx);
  await videoCtx.close();

  await browser.close();

  // Rename the video file (Playwright uses random names)
  const { readdirSync, renameSync } = await import('fs');
  const files = readdirSync(IMG_DIR).filter(f => f.endsWith('.webm') && f !== 'dashboard-walkthrough.webm');
  if (files.length > 0) {
    // Use the most recent webm file
    const newest = files.sort().pop();
    const src = `${IMG_DIR}/${newest}`;
    const dst = `${IMG_DIR}/dashboard-walkthrough.webm`;
    try { renameSync(dst, `${IMG_DIR}/dashboard-walkthrough-old.webm`); } catch {}
    renameSync(src, dst);
    console.log(`\n[video] Renamed ${newest} -> dashboard-walkthrough.webm`);
    // Clean up old file
    try { const { unlinkSync } = await import('fs'); unlinkSync(`${IMG_DIR}/dashboard-walkthrough-old.webm`); } catch {}
  }

  console.log('\nDone! All screenshots and walkthrough video captured.');
}

main().catch(err => {
  console.error('Fatal error:', err);
  process.exit(1);
});
