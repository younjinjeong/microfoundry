import { APIRequestContext } from '@playwright/test';

export class APIHelper {
  constructor(private request: APIRequestContext) {}

  async getApps() {
    const res = await this.request.get('/api/apps');
    return res.json();
  }

  async getApp(name: string) {
    const res = await this.request.get(`/api/apps/${name}`);
    return res.json();
  }

  async getServices() {
    const res = await this.request.get('/api/services');
    return res.json();
  }

  async getSecrets() {
    const res = await this.request.get('/api/secrets');
    return res.json();
  }

  async getClusters() {
    const res = await this.request.get('/api/clusters');
    return res.json();
  }

  async getCatalog() {
    const res = await this.request.get('/api/catalog');
    return res.json();
  }

  async getVisibleCatalog() {
    const res = await this.request.get('/api/catalog/visible');
    return res.json();
  }

  async getSettings() {
    const res = await this.request.get('/api/settings');
    return res.json();
  }

  async getWebhooks() {
    const res = await this.request.get('/api/settings/webhooks');
    return res.json();
  }

  async getAlerts() {
    const res = await this.request.get('/api/monitoring/alerts');
    return res.json();
  }

  async getOrgs() {
    const res = await this.request.get('/api/orgs');
    return res.json();
  }

  async getConfig() {
    const res = await this.request.get('/api/config');
    return res.json();
  }

  async getTopologies() {
    const res = await this.request.get('/api/topologies');
    return res.json();
  }
}
