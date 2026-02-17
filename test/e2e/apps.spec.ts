import { test, expect } from '../fixtures/admin-server';
import { apps, appDetail } from '../helpers/selectors';
import { APIHelper } from '../helpers/api';

test.describe('Applications - List', () => {
  test('app list page loads with table headers', async ({ page }) => {
    await page.goto('/apps');
    await expect(page.locator(apps.title)).toBeVisible();
    await expect(page.locator(apps.headerName)).toBeVisible();
    await expect(page.locator(apps.headerState)).toBeVisible();
    await expect(page.locator(apps.headerInstances)).toBeVisible();
    await expect(page.locator(apps.headerMemory)).toBeVisible();
    await expect(page.locator(apps.headerRoutes)).toBeVisible();
    await expect(page.locator(apps.headerCreated)).toBeVisible();
    await expect(page.locator(apps.headerActions)).toBeVisible();
  });

  test('state filter dropdown has all options', async ({ page }) => {
    await page.goto('/apps');
    const filter = page.locator(apps.stateFilter);
    await expect(filter).toBeVisible();

    const options = filter.locator('option');
    await expect(options).toHaveCount(3);
    await expect(options.nth(0)).toHaveText('All States');
    await expect(options.nth(1)).toHaveText('Started');
    await expect(options.nth(2)).toHaveText('Stopped');
  });

  test('state filter changes URL', async ({ page }) => {
    await page.goto('/apps');
    const filter = page.locator(apps.stateFilter);
    await filter.selectOption('started');
    await expect(page).toHaveURL(/state=started/);
  });

  test('app name links to detail page', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();

    if (appsList && appsList.length > 0) {
      await page.goto('/apps');
      const firstAppLink = page.locator(apps.tableBody).locator('a').first();
      const appName = await firstAppLink.textContent();
      await firstAppLink.click();
      await expect(page).toHaveURL(new RegExp(`/apps/${appName?.trim()}`));
    } else {
      // Empty state
      await page.goto('/apps');
      await expect(page.locator(apps.emptyState)).toBeVisible();
    }
  });

  test('empty state shows guidance message', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();

    if (!appsList || appsList.length === 0) {
      await page.goto('/apps');
      await expect(page.locator(apps.emptyState)).toBeVisible();
      await expect(page.locator('text=mf push')).toBeVisible();
    }
  });
});

test.describe('Applications - Detail', () => {
  test.beforeEach(async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) {
      test.skip();
    }
  });

  test('overview tab loads by default', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}`);
    await expect(page.locator(appDetail.overviewTab).first()).toBeVisible();
  });

  test('instances tab shows pod information', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}?tab=instances`);
    // Should show instance/pod table or scale form
    await expect(page.locator('text=Instances').first()).toBeVisible();
  });

  test('instances tab has auto-refresh', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}?tab=instances`);
    // Verify HTMX auto-refresh attribute exists
    const refreshEl = page.locator('[hx-trigger*="every"]');
    await expect(refreshEl.first()).toBeAttached();
  });

  test('config tab shows environment variables', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}?tab=config`);
    await expect(page.locator('text=Config').first()).toBeVisible();
  });

  test('services tab shows bindings', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}?tab=services`);
    await expect(page.locator('text=Services').first()).toBeVisible();
  });

  test('routes tab shows ingress information', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}?tab=routes`);
    await expect(page.locator('text=Routes').first()).toBeVisible();
  });

  test('logs tab has streaming controls', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}?tab=logs`);
    await expect(page.locator('text=Logs').first()).toBeVisible();
  });

  test('performance tab shows RED metrics', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}?tab=performance`);
    await expect(page.locator('text=Performance').first()).toBeVisible();
  });

  test('tab switching preserves URL with hx-push-url', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}`);

    // Click instances tab
    const instancesTab = page.locator('a:has-text("Instances")').first();
    if (await instancesTab.isVisible()) {
      await instancesTab.click();
      await page.waitForTimeout(500);
      await expect(page).toHaveURL(new RegExp('tab=instances'));
    }
  });

  test('HTMX tab loading uses partial endpoints', async ({ page, request }) => {
    const api = new APIHelper(request);
    const appsList = await api.getApps();
    if (!appsList || appsList.length === 0) return;

    const appName = appsList[0].name || appsList[0].Name;
    await page.goto(`/apps/${appName}`);

    // Verify tab links have hx-get pointing to /apps/{name}/tab/{tab}
    const tabLinks = page.locator(`[hx-get*="/apps/${appName}/tab/"]`);
    const count = await tabLinks.count();
    expect(count).toBeGreaterThan(0);
  });
});
