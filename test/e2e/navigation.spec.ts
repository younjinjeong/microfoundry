import { test, expect } from '../fixtures/admin-server';
import { nav } from '../helpers/selectors';

test.describe('Navigation', () => {
  test('sidebar renders Operations group with 5 nav items', async ({ page }) => {
    await page.goto('/');
    const sidebar = page.locator(nav.sidebar);
    await expect(sidebar).toBeVisible();

    // Operations section header
    await expect(page.locator(nav.operationsLabel).first()).toBeVisible();

    // Operations links
    await expect(sidebar.locator('a[href="/"]').first()).toBeVisible();
    await expect(sidebar.locator('a[href="/apps"]').first()).toBeVisible();
    await expect(sidebar.locator('a[href="/services"]').first()).toBeVisible();
    await expect(sidebar.locator('a[href="/secrets"]')).toBeVisible();
    await expect(sidebar.locator('a[href="/monitoring"]').first()).toBeVisible();
  });

  test('sidebar renders Settings group with nav items', async ({ page }) => {
    await page.goto('/');
    const sidebar = page.locator(nav.sidebar);

    // Settings section header
    await expect(page.locator(nav.settingsLabel).first()).toBeVisible();

    // Settings links
    await expect(sidebar.locator('a[href="/clusters"]')).toBeVisible();
    await expect(sidebar.locator('a[href="/catalog"]')).toBeVisible();
    await expect(sidebar.locator('a[href="/settings/registry"]')).toBeVisible();
    await expect(sidebar.locator('a[href="/settings/webhooks"]')).toBeVisible();
    await expect(sidebar.locator('a[href="/settings/smtp"]')).toBeVisible();
    await expect(sidebar.locator('a[href="/users"]')).toBeVisible();
    await expect(sidebar.locator('a[href="/config"]')).toBeVisible();
  });

  test('active state highlights current page', async ({ page }) => {
    // Dashboard active
    await page.goto('/');
    const dashLink = page.locator(nav.sidebar).locator('a[href="/"]').first();
    await expect(dashLink).toHaveClass(new RegExp(nav.activeClass));

    // Apps active
    await page.goto('/apps');
    const appsLink = page.locator(nav.apps).first();
    await expect(appsLink).toHaveClass(new RegExp(nav.activeClass));

    // Config active
    await page.goto('/config');
    const configLink = page.locator(nav.config);
    await expect(configLink).toHaveClass(new RegExp(nav.activeClass));
  });

  test('all nav links resolve to valid pages (no 404)', async ({ page }) => {
    const routes = [
      '/', '/apps', '/services', '/secrets', '/monitoring',
      '/clusters', '/catalog', '/settings/registry', '/settings/webhooks',
      '/settings/smtp', '/users', '/config',
    ];

    for (const route of routes) {
      const response = await page.goto(route);
      expect(response?.status(), `${route} should not return 404`).not.toBe(404);
    }
  });

  test('page titles match expected headings', async ({ page }) => {
    const pageTitles: Record<string, string> = {
      '/apps': 'Deployed Applications',
      '/services': 'Provisioned Services',
      '/secrets': 'Secrets',
      '/clusters': 'Kubernetes Clusters',
      '/catalog': 'Service Catalog',
      '/config': 'Kubernetes',
    };

    for (const [route, title] of Object.entries(pageTitles)) {
      await page.goto(route);
      await expect(page.locator(`text=${title}`).first()).toBeVisible();
    }
  });

  test('dashboard nav link returns to root', async ({ page }) => {
    await page.goto('/apps');
    // Click the Dashboard link in sidebar to return to /
    await page.locator(nav.sidebar).locator('a[href="/"]').first().click();
    await expect(page).toHaveURL('/');
  });
});
