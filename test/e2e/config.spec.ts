import { test, expect } from '../fixtures/admin-server';
import { config } from '../helpers/selectors';

test.describe('Platform Config', () => {
  test('config page loads with sections', async ({ page }) => {
    await page.goto('/config');
    await expect(page.locator(config.kubernetesSection)).toBeVisible();
    await expect(page.locator(config.githubSection)).toBeVisible();
    await expect(page.locator(config.platformSection)).toBeVisible();
  });

  test('config sections display key-value pairs', async ({ page }) => {
    await page.goto('/config');

    // Kubernetes section should show context, namespace, domain
    await expect(page.locator('dt:has-text("Context")')).toBeVisible();
    await expect(page.locator('dt:has-text("Namespace")')).toBeVisible();
    await expect(page.locator('dt:has-text("Domain")')).toBeVisible();

    // Values should be in monospace font
    const monoValues = page.locator('dd.font-mono');
    const count = await monoValues.count();
    expect(count).toBeGreaterThan(0);
  });
});
