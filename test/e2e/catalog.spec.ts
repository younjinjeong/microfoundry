import { test, expect } from '../fixtures/admin-server';
import { catalog } from '../helpers/selectors';

test.describe('Service Catalog', () => {
  test('catalog page loads with service types', async ({ page }) => {
    await page.goto('/catalog');
    await expect(page.locator(catalog.title)).toBeVisible();
  });

  test('all 10 service types are displayed', async ({ page }) => {
    await page.goto('/catalog');
    const serviceTypes = [
      'MariaDB', 'PostgreSQL', 'ClickHouse',
      'Redis', 'Memcached',
      'RabbitMQ', 'ActiveMQ',
      'MinIO',
      'Kong', 'NGINX',
    ];

    for (const svcType of serviceTypes) {
      await expect(page.locator(`text=${svcType}`).first()).toBeVisible();
    }
  });

  test('category grouping is present', async ({ page }) => {
    await page.goto('/catalog');
    await expect(page.locator(catalog.databases)).toBeVisible();
    await expect(page.locator(catalog.caches)).toBeVisible();
    await expect(page.locator(catalog.messaging)).toBeVisible();
  });

  test('plan details show resource information', async ({ page }) => {
    await page.goto('/catalog');
    // Plans should show small/medium/large with resource specs
    await expect(page.locator('text=small').first()).toBeVisible();
    await expect(page.locator('text=medium').first()).toBeVisible();
    await expect(page.locator('text=large').first()).toBeVisible();
  });

  test('visibility toggle buttons are present', async ({ page }) => {
    await page.goto('/catalog');
    // Visibility toggle buttons should exist
    const toggleButtons = page.locator('button[hx-post*="visibility"]');
    const count = await toggleButtons.count();
    expect(count).toBeGreaterThan(0);
  });

  test('topology page loads for a service plan', async ({ page }) => {
    // Navigate to a topology page for mariadb/small
    const response = await page.goto('/topologies/mariadb/small');
    expect(response?.status()).not.toBe(404);
  });
});
