import { chromium, Browser } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';
// @ts-ignore — no type declarations for gif-encoder-2
import GIFEncoder from 'gif-encoder-2';
import { PNG } from 'pngjs';

const BASE_URL = 'https://admin.cf-local.dev:8443';
const OUTPUT_DIR = path.resolve(__dirname, '../docs/images');

// Keycloak credentials (platform-admin)
const KC_USER = 'myadmin';
const KC_PASS = 'Admin123!';

// GIF settings
const GIF_WIDTH = 1440;
const GIF_HEIGHT = 900;
const GIF_FRAME_DELAY = 1500; // ms between frames

// All pages to screenshot (super admin view — full menu)
const pages = [
  // Operations
  { name: 'dashboard',          url: '/',                          title: 'Dashboard' },
  { name: 'apps-list',          url: '/apps',                      title: 'Applications' },
  { name: 'services',           url: '/services',                  title: 'Services' },
  { name: 'secrets',            url: '/secrets',                   title: 'Secrets' },
  // Settings
  { name: 'users-iam',          url: '/users',                     title: 'Users & IAM' },
  { name: 'workspaces',         url: '/workspaces',                title: 'Workspaces' },
  { name: 'clusters',           url: '/clusters',                  title: 'Clusters' },
  { name: 'catalog',            url: '/catalog',                   title: 'Service Catalog' },
  { name: 'settings-registry',  url: '/settings/registry',         title: 'Registry Settings' },
  { name: 'settings-webhooks',  url: '/settings/webhooks',         title: 'Webhook Settings' },
  { name: 'settings-smtp',      url: '/settings/smtp',             title: 'SMTP Settings' },
  { name: 'settings-endpoints', url: '/settings/endpoints',        title: 'Endpoint Settings' },
  { name: 'settings-csp',       url: '/settings/cloud-providers',  title: 'Cloud Providers' },
  { name: 'monitoring',         url: '/monitoring',                title: 'Monitoring & Alerts' },
  { name: 'config',             url: '/config',                    title: 'Platform Config' },
  { name: 'docs',               url: '/docs',                     title: 'Documentation' },
];

// Full super-admin walkthrough — every menu and sub-tab
interface WalkthroughStep {
  url: string;
  wait: number;
  desc: string;
  scroll?: boolean;
  clickTab?: string;
}

const walkthrough: WalkthroughStep[] = [
  // Operations
  { url: '/',                      wait: 2500, desc: 'Dashboard' },
  { url: '/apps',                  wait: 2000, desc: 'Applications' },
  { url: '/services',              wait: 2000, desc: 'Services' },
  { url: '/secrets',               wait: 2000, desc: 'Secrets' },

  // IAM — walk through all tabs
  { url: '/users',                 wait: 2000, desc: 'Users & IAM (Workspaces tab)' },
  { url: '/users?tab=orgs',       wait: 2000, desc: 'IAM — Organizations' },
  { url: '/users?tab=users',      wait: 2000, desc: 'IAM — Users' },
  { url: '/users?tab=policies',   wait: 2000, desc: 'IAM — Policies' },
  { url: '/users?tab=audit',      wait: 2000, desc: 'IAM — Audit Log' },

  // Workspaces
  { url: '/workspaces',            wait: 2000, desc: 'Workspaces' },

  // Settings
  { url: '/clusters',              wait: 2000, desc: 'Clusters' },

  // Service Catalog — cycle through provider tabs
  { url: '/catalog',               wait: 2000, desc: 'Catalog — All' },
  { url: '/catalog',               wait: 1500, desc: 'Catalog — AWS',   clickTab: '.provider-tab[data-provider-tab="aws"]' },
  { url: '/catalog',               wait: 1500, desc: 'Catalog — GCP',   clickTab: '.provider-tab[data-provider-tab="gcp"]' },
  { url: '/catalog',               wait: 1500, desc: 'Catalog — Azure', clickTab: '.provider-tab[data-provider-tab="azure"]' },
  { url: '/catalog',               wait: 1500, desc: 'Catalog — Local', clickTab: '.provider-tab[data-provider-tab="local"]', scroll: true },

  { url: '/settings/registry',     wait: 1500, desc: 'Registry Settings' },
  { url: '/settings/webhooks',     wait: 1500, desc: 'Webhook Settings' },
  { url: '/settings/smtp',         wait: 1500, desc: 'SMTP Settings' },
  { url: '/settings/endpoints',    wait: 2000, desc: 'Endpoint Settings' },
  { url: '/settings/cloud-providers', wait: 2000, desc: 'Cloud Provider Settings' },
  { url: '/monitoring',            wait: 2000, desc: 'Monitoring & Alerts' },
  { url: '/config',                wait: 2000, desc: 'Platform Config' },
  { url: '/docs',                  wait: 2000, desc: 'Documentation (User Manual)' },
  { url: '/docs?tab=admin',        wait: 2000, desc: 'Documentation (Admin Guide)' },
  { url: '/docs?tab=architecture', wait: 2000, desc: 'Documentation (Architecture)' },

  // Return to dashboard
  { url: '/',                      wait: 2000, desc: 'Back to Dashboard' },
];

async function signIn(page: import('@playwright/test').Page) {
  console.log('  🔐 Signing in as platform-admin...');

  // Navigate to admin — will redirect to login page
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);

  // Click "Sign in with Keycloak" button (redirects to /auth/login → Keycloak)
  const signInBtn = page.locator('text=Sign in with Keycloak');
  if (await signInBtn.isVisible()) {
    await signInBtn.click();
  } else {
    const headerSignIn = page.locator('a[href="/auth/login"]');
    if (await headerSignIn.isVisible()) {
      await headerSignIn.click();
    }
  }

  // Wait for Keycloak login form to appear
  const usernameField = page.locator('#username');
  await usernameField.waitFor({ state: 'visible', timeout: 15000 });
  console.log('  📝 Keycloak login form loaded');

  // Fill and submit
  await usernameField.fill(KC_USER);
  await page.locator('#password').fill(KC_PASS);
  await page.locator('#kc-login').click();
  console.log('  📝 Submitted credentials, waiting for redirect...');

  // Wait for Keycloak to process — it may show intermediate pages
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(2000);

  // Handle any intermediate Keycloak pages (required actions, consent, etc.)
  for (let attempt = 0; attempt < 5; attempt++) {
    const currentUrl = page.url();
    if (currentUrl.startsWith(BASE_URL)) {
      break; // We're back on the admin site
    }

    console.log(`  🔄 Still on Keycloak (attempt ${attempt + 1}): ${currentUrl.substring(0, 80)}...`);

    // Check for common Keycloak required action buttons
    const submitBtn = page.locator('input[type="submit"], button[type="submit"]');
    if (await submitBtn.first().isVisible({ timeout: 2000 })) {
      console.log('  📝 Clicking through Keycloak intermediate page...');
      await submitBtn.first().click();
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(2000);
    } else {
      await page.waitForTimeout(2000);
    }
  }

  // Final check — navigate to dashboard if not already there
  const currentUrl = page.url();
  if (!currentUrl.startsWith(BASE_URL)) {
    console.log('  ⚠️ Still not on admin site, navigating directly...');
    await page.goto(BASE_URL, { waitUntil: 'networkidle' });
    await page.waitForTimeout(2000);
  }

  // Verify sign-in succeeded — authenticated nav shows admin-only links
  const adminNav = page.locator('a[href="/users"]');
  if (await adminNav.isVisible({ timeout: 5000 })) {
    console.log('  ✅ Signed in as platform-admin (admin nav verified)');
  } else {
    console.log('  ⚠️ Sign-in may have failed — admin nav not visible, retrying...');
    // Try the full sign-in flow once more
    await page.goto(BASE_URL + '/auth/login', { waitUntil: 'networkidle' });
    await page.waitForTimeout(2000);
    const retryUsername = page.locator('#username');
    if (await retryUsername.isVisible({ timeout: 5000 })) {
      await retryUsername.fill(KC_USER);
      await page.locator('#password').fill(KC_PASS);
      await page.locator('#kc-login').click();
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(5000);
      await page.goto(BASE_URL, { waitUntil: 'networkidle' });
      await page.waitForTimeout(2000);
    }
    if (await adminNav.isVisible({ timeout: 3000 })) {
      console.log('  ✅ Signed in on retry');
    } else {
      console.log('  ❌ Could not verify sign-in — continuing anyway');
    }
  }
}

async function takeScreenshots(browser: Browser) {
  console.log('\n📸 Taking screenshots (super admin)...\n');
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();

  // Sign in first
  await signIn(page);

  for (const p of pages) {
    const filepath = path.join(OUTPUT_DIR, `${p.name}.png`);
    await page.goto(`${BASE_URL}${p.url}`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(500);
    await page.screenshot({ path: filepath, fullPage: false });
    console.log(`  ✅ ${p.title} → ${p.name}.png`);
  }

  await context.close();
  console.log(`\n📸 ${pages.length} screenshots saved to docs/images/\n`);
}

async function recordWalkthrough(browser: Browser) {
  console.log('\n🎬 Recording walkthrough + generating GIF (super admin)...\n');

  // Use recordVideo for .webm and capture screenshots for GIF
  const context = await browser.newContext({
    viewport: { width: GIF_WIDTH, height: GIF_HEIGHT },
    deviceScaleFactor: 1,
    ignoreHTTPSErrors: true,
    recordVideo: {
      dir: OUTPUT_DIR,
      size: { width: GIF_WIDTH, height: GIF_HEIGHT },
    },
  });
  const page = await context.newPage();

  // Sign in first
  await signIn(page);

  // Collect PNG buffers for GIF generation
  const frames: Buffer[] = [];
  let lastUrl = '';

  for (const step of walkthrough) {
    console.log(`  🎥 ${step.desc}...`);

    // Only navigate if URL changed (for tab clicking on same page)
    if (step.url !== lastUrl) {
      await page.goto(`${BASE_URL}${step.url}`, { waitUntil: 'networkidle' });
      await page.waitForTimeout(300);
    }
    lastUrl = step.url;

    // Click tab if specified (for catalog provider tabs)
    if (step.clickTab) {
      const tab = page.locator(step.clickTab);
      if (await tab.isVisible({ timeout: 2000 })) {
        await tab.click();
        await page.waitForTimeout(500);
      }
    }

    if (step.scroll) {
      await page.evaluate(() => {
        return new Promise<void>((resolve) => {
          let totalHeight = 0;
          const distance = 200;
          const timer = setInterval(() => {
            window.scrollBy(0, distance);
            totalHeight += distance;
            if (totalHeight >= document.body.scrollHeight - window.innerHeight) {
              clearInterval(timer);
              setTimeout(() => {
                window.scrollTo(0, 0);
                resolve();
              }, 500);
            }
          }, 100);
        });
      });
    }

    await page.waitForTimeout(step.wait);

    // Capture frame for GIF
    const screenshotBuffer = await page.screenshot({ type: 'png' });
    frames.push(screenshotBuffer);
  }

  await page.close();
  await context.close();

  // Rename the recorded .webm video
  const webmFiles = fs.readdirSync(OUTPUT_DIR).filter((f: string) => f.endsWith('.webm') && f !== 'dashboard-walkthrough.webm');
  if (webmFiles.length > 0) {
    const latestVideo = webmFiles.sort().pop()!;
    const src = path.join(OUTPUT_DIR, latestVideo);
    const dest = path.join(OUTPUT_DIR, 'dashboard-walkthrough.webm');
    if (fs.existsSync(dest)) {
      fs.unlinkSync(dest);
    }
    fs.renameSync(src, dest);
    console.log(`\n🎬 Walkthrough video saved → docs/images/dashboard-walkthrough.webm`);
  }

  // Generate GIF from collected frames
  console.log(`\n🖼️  Generating GIF from ${frames.length} frames...`);
  await generateGif(frames);
}

async function generateGif(frames: Buffer[]) {
  const encoder = new GIFEncoder(GIF_WIDTH, GIF_HEIGHT, 'neuquant', true);
  const gifPath = path.join(OUTPUT_DIR, 'dashboard-walkthrough.gif');

  encoder.setDelay(GIF_FRAME_DELAY);
  encoder.setRepeat(0); // infinite loop
  encoder.setQuality(10); // lower = better quality, slower

  const writeStream = fs.createWriteStream(gifPath);
  encoder.createReadStream().pipe(writeStream);
  encoder.start();

  for (let i = 0; i < frames.length; i++) {
    const png = PNG.sync.read(frames[i]);
    // Resize if screenshot is larger than GIF dimensions (deviceScaleFactor)
    if (png.width !== GIF_WIDTH || png.height !== GIF_HEIGHT) {
      // Create a simple nearest-neighbor downscale
      const scaled = new Uint8Array(GIF_WIDTH * GIF_HEIGHT * 4);
      const scaleX = png.width / GIF_WIDTH;
      const scaleY = png.height / GIF_HEIGHT;
      for (let y = 0; y < GIF_HEIGHT; y++) {
        for (let x = 0; x < GIF_WIDTH; x++) {
          const srcX = Math.floor(x * scaleX);
          const srcY = Math.floor(y * scaleY);
          const srcIdx = (srcY * png.width + srcX) * 4;
          const dstIdx = (y * GIF_WIDTH + x) * 4;
          scaled[dstIdx] = png.data[srcIdx];
          scaled[dstIdx + 1] = png.data[srcIdx + 1];
          scaled[dstIdx + 2] = png.data[srcIdx + 2];
          scaled[dstIdx + 3] = png.data[srcIdx + 3];
        }
      }
      encoder.addFrame(scaled as any);
    } else {
      encoder.addFrame(png.data as any);
    }
    console.log(`  Frame ${i + 1}/${frames.length}`);
  }

  encoder.finish();

  // Wait for write stream to finish
  await new Promise<void>((resolve, reject) => {
    writeStream.on('finish', resolve);
    writeStream.on('error', reject);
  });

  const stats = fs.statSync(gifPath);
  console.log(`\n🎬 GIF saved → docs/images/dashboard-walkthrough.gif (${(stats.size / 1024 / 1024).toFixed(1)} MB)\n`);
}

async function main() {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });

  const browser = await chromium.launch({ headless: true });

  try {
    await takeScreenshots(browser);
    await recordWalkthrough(browser);
  } finally {
    await browser.close();
  }

  console.log('✨ Done! All screenshots, video, and GIF are in docs/images/');
}

main().catch(console.error);
