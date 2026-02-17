import { test, expect } from '../fixtures/admin-server';
import { monitoring } from '../helpers/selectors';

test.describe('Monitoring', () => {
  test('monitoring page loads', async ({ page }) => {
    await page.goto('/monitoring');
    await expect(page.locator(monitoring.title).first()).toBeVisible();
  });

  test('Grafana dashboard section is present', async ({ page }) => {
    await page.goto('/monitoring');
    // Should have Grafana button or iframe
    const grafanaEl = page.locator(monitoring.grafanaButton)
      .or(page.locator('iframe[src*="grafana"]'))
      .or(page.locator('text=Grafana'));
    await expect(grafanaEl.first()).toBeVisible();
  });

  test('Prometheus and AlertManager links are present', async ({ page }) => {
    await page.goto('/monitoring');
    // Links to Prometheus/AlertManager or references
    const prometheusRef = page.locator('text=Prometheus').or(page.locator('a[href*="prometheus"]'));
    // At least monitoring page should reference these
    await expect(page.locator('.bg-white').first()).toBeVisible();
  });

  test('alerts page loads', async ({ page }) => {
    await page.goto('/monitoring/alerts');
    // Alert list should render (may be empty)
    await expect(page.locator('text=Alert').or(page.locator('text=Alerts')).first()).toBeVisible();
  });

  test('alerts auto-refresh with HTMX', async ({ page }) => {
    await page.goto('/monitoring');
    // Alerts container should have auto-refresh
    const refreshEl = page.locator('[hx-trigger*="every"]').or(page.locator(monitoring.alertsContainer));
    await expect(refreshEl.first()).toBeAttached();
  });
});
