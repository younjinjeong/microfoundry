import { test as base, expect } from '@playwright/test';

export const test = base.extend<{ serverReady: void }>({
  serverReady: [async ({ page }, use) => {
    // Wait for admin server to be ready
    const maxRetries = 60;
    const retryInterval = 500;
    let ready = false;

    for (let i = 0; i < maxRetries; i++) {
      try {
        const response = await page.request.get('/');
        if (response.ok()) {
          ready = true;
          break;
        }
      } catch {
        // Server not ready yet
      }
      await page.waitForTimeout(retryInterval);
    }

    expect(ready, 'Admin server should be ready').toBe(true);
    await use();
  }, { auto: true }],
});

export { expect };
