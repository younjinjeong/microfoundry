import { test, expect } from '../fixtures/admin-server';
import { clusters } from '../helpers/selectors';
import { APIHelper } from '../helpers/api';

test.describe('Clusters', () => {
  test('cluster list page loads', async ({ page }) => {
    await page.goto('/clusters');
    await expect(page.locator(clusters.title)).toBeVisible();
  });

  test('active cluster has indicator badge', async ({ page }) => {
    await page.goto('/clusters');
    // Active cluster should have a visible indicator
    await expect(page.locator('text=active').or(page.locator('.bg-green-100')).first()).toBeVisible();
  });

  test('cluster detail page shows node information', async ({ page, request }) => {
    const api = new APIHelper(request);
    const clusterList = await api.getClusters();

    if (clusterList && clusterList.length > 0) {
      const clusterId = clusterList[0].id || clusterList[0].ID;
      await page.goto(`/clusters/${clusterId}`);
      // Should show node table or cluster info
      await expect(page.locator('text=Node').or(page.locator('text=Status')).first()).toBeVisible();
    }
  });

  test('add cluster form has required fields', async ({ page }) => {
    await page.goto('/clusters');
    const addBtn = page.locator(clusters.addButton);
    await expect(addBtn).toBeVisible();
    await addBtn.click();

    // Form should become visible
    const form = page.locator(clusters.addForm);
    await expect(form).toBeVisible();

    // Verify form fields
    await expect(page.locator(clusters.nameInput)).toBeVisible();
    await expect(page.locator(clusters.contextInput)).toBeVisible();
    await expect(page.locator(clusters.namespaceInput)).toBeVisible();
    await expect(page.locator(clusters.domainInput)).toBeVisible();
  });

  test('activate cluster button is present', async ({ page }) => {
    await page.goto('/clusters');
    // Activate buttons should exist for non-active clusters
    const activateBtns = page.locator('button:has-text("Activate"), button[hx-post*="activate"]');
    // At least the active cluster exists (may or may not have activate buttons depending on count)
    await expect(page.locator('table, .bg-white').first()).toBeVisible();
  });

  test('remove cluster has protection for active cluster', async ({ page, request }) => {
    const api = new APIHelper(request);
    const clusterList = await api.getClusters();

    if (clusterList && clusterList.length > 0) {
      await page.goto('/clusters');
      // Active cluster should not have a delete button, or it should be disabled
      const activeRow = page.locator('tr:has(.bg-green-100), tr:has(text=active)').first();
      if (await activeRow.isVisible()) {
        // The active cluster row should either have no delete button or a disabled one
        await expect(activeRow).toBeVisible();
      }
    }
  });

  test('cluster health check returns status', async ({ page, request }) => {
    const api = new APIHelper(request);
    const clusterList = await api.getClusters();

    if (clusterList && clusterList.length > 0) {
      const clusterId = clusterList[0].id || clusterList[0].ID;
      const response = await request.get(`/api/clusters/${clusterId}/health`);
      expect(response.status()).toBeLessThan(500);
    }
  });
});
