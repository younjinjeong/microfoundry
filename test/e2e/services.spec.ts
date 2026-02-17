import { test, expect } from '../fixtures/admin-server';
import { services } from '../helpers/selectors';
import { APIHelper } from '../helpers/api';

test.describe('Services', () => {
  test('service list page loads', async ({ page }) => {
    await page.goto('/services');
    await expect(page.locator(services.title)).toBeVisible();
  });

  test('create service button links to catalog', async ({ page }) => {
    await page.goto('/services');
    const createBtn = page.locator(services.createButton);
    await expect(createBtn).toBeVisible();
    await createBtn.click();
    await expect(page).toHaveURL('/catalog');
  });

  test('service detail page loads', async ({ page, request }) => {
    const api = new APIHelper(request);
    const svcList = await api.getServices();

    if (svcList && svcList.length > 0) {
      const svcName = svcList[0].name || svcList[0].Name;
      await page.goto(`/services/${svcName}`);
      await expect(page.locator(`text=${svcName}`).first()).toBeVisible();
    } else {
      await page.goto('/services');
      await expect(page.locator(services.emptyState)).toBeVisible();
    }
  });

  test('service list shows table or empty state', async ({ page, request }) => {
    const api = new APIHelper(request);
    const svcList = await api.getServices();

    await page.goto('/services');
    if (svcList && svcList.length > 0) {
      await expect(page.locator(services.tableBody)).toBeVisible();
    } else {
      await expect(page.locator(services.emptyState)).toBeVisible();
    }
  });
});
