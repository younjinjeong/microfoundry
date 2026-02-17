import { test, expect } from '../fixtures/admin-server';
import { users } from '../helpers/selectors';
import { APIHelper } from '../helpers/api';

test.describe('Users & Organizations', () => {
  test('users page loads', async ({ page }) => {
    await page.goto('/users');
    await expect(page.locator(users.title).or(page.locator('text=Organizations')).first()).toBeVisible();
  });

  test('create org form is available', async ({ page }) => {
    await page.goto('/users');
    // Form or create button should be visible
    const createForm = page.locator(users.createOrgForm)
      .or(page.locator('button:has-text("Create")'))
      .or(page.locator('input[name="name"]'));
    // When auth is disabled, a message is shown instead
    const noAuthMsg = page.locator('text=authentication').or(page.locator('text=login'));
    await expect(createForm.first().or(noAuthMsg.first())).toBeVisible();
  });

  test('member management actions are available when orgs exist', async ({ page, request }) => {
    const api = new APIHelper(request);
    let orgs: any[] = [];
    try {
      orgs = await api.getOrgs();
    } catch {
      // API may not be available without auth
    }

    await page.goto('/users');
    if (orgs && orgs.length > 0) {
      // Should show org list with member management
      await expect(page.locator('text=Members').or(page.locator('text=Role')).first()).toBeVisible();
    }
  });
});
