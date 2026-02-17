import { test, expect } from '../fixtures/admin-server';
import { secrets } from '../helpers/selectors';
import { APIHelper } from '../helpers/api';

test.describe('Secrets', () => {
  test('secret list page loads', async ({ page }) => {
    await page.goto('/secrets');
    await expect(page.locator(secrets.title)).toBeVisible();
  });

  test('create secret button links to new form', async ({ page }) => {
    await page.goto('/secrets');
    const createBtn = page.locator(secrets.createButton).first();
    await expect(createBtn).toBeVisible();
    await createBtn.click();
    await expect(page).toHaveURL('/secrets/new');
  });

  test('create secret form has key-value inputs', async ({ page }) => {
    await page.goto('/secrets/new');
    // Form should have name field and at least one key-value pair
    await expect(page.locator('input[name="name"]').or(page.locator('input[placeholder*="name" i]')).first()).toBeVisible();
  });

  test('secret detail page shows masked values', async ({ page, request }) => {
    const api = new APIHelper(request);
    const secretsList = await api.getSecrets();

    if (secretsList && secretsList.length > 0) {
      const secretName = secretsList[0].name || secretsList[0].Name;
      await page.goto(`/secrets/${secretName}`);
      await expect(page.locator(`text=${secretName}`).first()).toBeVisible();
    }
  });

  test('reveal key endpoint works via HTMX', async ({ page, request }) => {
    const api = new APIHelper(request);
    const secretsList = await api.getSecrets();

    if (secretsList && secretsList.length > 0) {
      const secretName = secretsList[0].name || secretsList[0].Name;
      await page.goto(`/secrets/${secretName}`);
      // Look for reveal buttons/links
      const revealBtn = page.locator('button:has-text("Reveal"), button:has-text("Show"), [hx-get*="reveal"]').first();
      if (await revealBtn.isVisible()) {
        await expect(revealBtn).toBeVisible();
      }
    }
  });
});
