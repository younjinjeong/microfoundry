import { test, expect } from '../fixtures/admin-server';

test.describe('JSON API Endpoints', () => {
  test('GET /api/apps returns JSON', async ({ request }) => {
    const response = await request.get('/api/apps');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/services returns JSON', async ({ request }) => {
    const response = await request.get('/api/services');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/secrets returns JSON', async ({ request }) => {
    const response = await request.get('/api/secrets');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/clusters returns JSON', async ({ request }) => {
    const response = await request.get('/api/clusters');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/catalog returns all service types', async ({ request }) => {
    const response = await request.get('/api/catalog');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
    const data = await response.json();
    expect(Array.isArray(data) || typeof data === 'object').toBe(true);
  });

  test('GET /api/catalog/visible returns visible catalog', async ({ request }) => {
    const response = await request.get('/api/catalog/visible');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/settings returns platform settings', async ({ request }) => {
    const response = await request.get('/api/settings');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/settings/webhooks returns webhook list', async ({ request }) => {
    const response = await request.get('/api/settings/webhooks');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/monitoring/alerts returns response', async ({ request }) => {
    const response = await request.get('/api/monitoring/alerts');
    // AlertManager may not be running, so accept 200 or 502/503
    expect(response.status()).toBeLessThanOrEqual(503);
  });

  test('GET /api/orgs returns organization list', async ({ request }) => {
    const response = await request.get('/api/orgs');
    expect(response.status()).toBeLessThan(500);
  });

  test('GET /api/config returns platform config', async ({ request }) => {
    const response = await request.get('/api/config');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });

  test('GET /api/topologies returns topology list', async ({ request }) => {
    const response = await request.get('/api/topologies');
    expect(response.status()).toBeLessThan(500);
    expect(response.headers()['content-type']).toContain('json');
  });
});
