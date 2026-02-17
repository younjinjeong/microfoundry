import { test, expect } from '../fixtures/admin-server';
import { dashboard } from '../helpers/selectors';

test.describe('Dashboard', () => {
  test('dashboard loads at root URL with stat cards', async ({ page }) => {
    await page.goto('/');
    // Four stat cards visible — use p.text-sm to target card labels (avoids matching nav links)
    await expect(page.locator('.bg-white.rounded-lg.shadow').first()).toBeVisible();
    await expect(page.locator('p.text-sm:has-text("Applications")')).toBeVisible();
    await expect(page.locator('p.text-sm:has-text("Domain")')).toBeVisible();
    await expect(page.locator('p.text-sm:has-text("Namespace")')).toBeVisible();
    await expect(page.locator('p.text-sm:has-text("K8s Context")')).toBeVisible();
  });

  test('application count card displays a number', async ({ page }) => {
    await page.goto('/');
    const appCount = page.locator(dashboard.appCountCard);
    await expect(appCount).toBeVisible();
    const text = await appCount.textContent();
    expect(text?.trim()).toMatch(/^\d+$/);
  });

  test('cluster info cards display values', async ({ page }) => {
    await page.goto('/');
    // Domain card should have a non-empty value
    const domainValue = page.locator('.text-lg.font-bold.text-gray-900').nth(0);
    await expect(domainValue).toBeVisible();
  });

  test('quick action links navigate correctly', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator(dashboard.quickLinks)).toBeVisible();

    // View Applications link
    const appsLink = page.locator(dashboard.viewAppsLink);
    await expect(appsLink).toBeVisible();
    await appsLink.click();
    await expect(page).toHaveURL('/apps');

    // Go back and test Platform Configuration link
    await page.goto('/');
    const configLink = page.locator(dashboard.platformConfigLink);
    await expect(configLink).toBeVisible();
    await configLink.click();
    await expect(page).toHaveURL('/config');

    // Go back and test Backing Services link
    await page.goto('/');
    const servicesLink = page.locator(dashboard.backingServicesLink);
    await expect(servicesLink).toBeVisible();
    await servicesLink.click();
    await expect(page).toHaveURL('/services');
  });
});
