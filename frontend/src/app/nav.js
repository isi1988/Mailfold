// Navigation model for the sidebar. Keys double as route segments ("/mailboxes")
// and as i18n keys (nav.<key> for items, nav.group.<key> for section headers).
// Labels are resolved at render time so they follow the active language.
export const NAV = [
  { key: 'dashboard' },
  { group: 'mail' },
  { key: 'mailboxes' },
  { key: 'domains' },
  { key: 'aliases' },
  { group: 'flow' },
  { key: 'queue' },
  { key: 'quarantine' },
  { key: 'spam' },
  { group: 'system' },
  { key: 'syncjobs' },
  { key: 'logs' },
  { key: 'settings' },
  { group: 'apps' },
  { key: 'webmail' },
  { key: 'apikeys' },
];

// Route keys that render inside the standard content area.
export const PAGE_KEYS = [
  'dashboard', 'mailboxes', 'domains', 'aliases', 'queue',
  'quarantine', 'spam', 'syncjobs', 'logs', 'webmail', 'settings', 'apikeys',
];
