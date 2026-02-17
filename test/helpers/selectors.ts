// CSS selectors for MicroFoundry admin pages
// Centralized for easy maintenance when templates change

export const nav = {
  sidebar: 'aside',
  brand: 'aside h1',
  operationsLabel: 'text=Operations',
  settingsLabel: 'text=Settings',
  // Operations links
  dashboard: 'a[href="/"]',
  apps: 'a[href="/apps"]',
  services: 'a[href="/services"]',
  secrets: 'a[href="/secrets"]',
  monitoring: 'a[href="/monitoring"]',
  // Settings links
  clusters: 'a[href="/clusters"]',
  catalog: 'a[href="/catalog"]',
  registry: 'a[href="/settings/registry"]',
  webhooks: 'a[href="/settings/webhooks"]',
  smtp: 'a[href="/settings/smtp"]',
  users: 'a[href="/users"]',
  config: 'a[href="/config"]',
  activeClass: 'bg-gray-800',
};

export const dashboard = {
  appCountCard: 'a[href="/apps"] .text-2xl',
  domainCard: 'text=Domain',
  namespaceCard: 'text=Namespace',
  contextCard: 'text=K8s Context',
  quickLinks: 'text=Quick Links',
  viewAppsLink: 'text=View Applications',
  platformConfigLink: 'text=Platform Configuration',
  backingServicesLink: 'text=Backing Services',
};

export const apps = {
  title: 'text=Deployed Applications',
  stateFilter: '#state-filter',
  refreshButton: 'button:has-text("Refresh")',
  tableBody: '#app-table-body',
  emptyState: 'text=No applications deployed',
  // Table headers
  headerName: 'th:has-text("Name")',
  headerState: 'th:has-text("State")',
  headerInstances: 'th:has-text("Instances")',
  headerMemory: 'th:has-text("Memory")',
  headerRoutes: 'th:has-text("Routes")',
  headerCreated: 'th:has-text("Created")',
  headerActions: 'th:has-text("Actions")',
};

export const appDetail = {
  // Tabs
  overviewTab: 'text=Overview',
  instancesTab: 'text=Instances',
  configTab: 'text=Config',
  servicesTab: 'text=Services',
  routesTab: 'text=Routes',
  logsTab: 'text=Logs',
  performanceTab: 'text=Performance',
};

export const services = {
  title: 'text=Provisioned Services',
  createButton: 'a:has-text("Create Service")',
  tableBody: '#service-table-body',
  emptyState: 'text=No services provisioned',
};

export const catalog = {
  title: 'header h2',
  uploadTopology: 'text=Upload Topology',
  // Categories — use .first() in tests since category text may appear multiple times
  databases: 'h3:has-text("Databases")',
  caches: 'h3:has-text("Caches")',
  messaging: 'h3:has-text("Messaging")',
  storage: 'h3:has-text("Storage")',
  gateway: 'h3:has-text("Gateways")',
};

export const secrets = {
  title: 'header h2',
  createButton: 'a[href="/secrets/new"]',
  secretList: '#secret-list',
  filterAll: 'text=All',
  filterService: 'text=Service',
  filterUser: 'text=User-defined',
};

export const clusters = {
  title: 'text=Kubernetes Clusters',
  addButton: 'button:has-text("Add Cluster")',
  addForm: '#add-cluster-form',
  nameInput: '#cluster-name',
  providerInput: '#cluster-provider',
  contextInput: '#cluster-context',
  namespaceInput: '#cluster-namespace',
  domainInput: '#cluster-domain',
  regionInput: '#cluster-region',
};

export const settingsRegistry = {
  title: 'text=Container Registry',
  urlInput: '#reg-url',
  projectInput: '#reg-project',
  usernameInput: '#reg-username',
  passwordInput: '#reg-password',
  insecureCheckbox: 'input[name="insecure"]',
  enabledCheckbox: 'input[name="enabled"]',
  testButton: 'button:has-text("Test Connection")',
  saveButton: 'button:has-text("Save")',
  successBanner: 'text=Settings saved',
};

export const settingsWebhooks = {
  title: 'header h2',
  addButton: 'button:has-text("Add Webhook")',
  addForm: '#add-webhook-form',
  nameInput: '#wh-name',
  urlInput: '#wh-url',
  eventsCheckboxes: 'input[name="events"]',
};

export const settingsSMTP = {
  title: 'header h2',
  hostInput: '#smtp-host',
  portInput: '#smtp-port',
  usernameInput: '#smtp-username',
  passwordInput: '#smtp-password',
  fromInput: '#smtp-from',
  tlsCheckbox: 'input[name="tls"]',
  enabledCheckbox: 'input[name="enabled"]',
  testButton: 'button:has-text("Test Connection")',
  saveButton: 'button:has-text("Save")',
};

export const monitoring = {
  title: 'text=Metrics & Alerts',
  grafanaButton: 'text=Open Grafana',
  alertsContainer: '#alerts-container',
};

export const users = {
  title: 'text=Users & Organizations',
  createOrgForm: '#create-org-form',
};

export const config = {
  kubernetesSection: 'h3:has-text("Kubernetes")',
  githubSection: 'h3:has-text("GitHub")',
  platformSection: 'h3:has-text("Platform")',
};
