// ============================================================
// Mailfold — Sample data
// All demo content for the pages, in one place. Plain ES module.
// ============================================================

// initials from a display name, e.g. "Ana Ruiz" -> "AR"
export function initials(name) {
  return String(name).split(' ').map(w => w[0]).join('').slice(0, 2).toUpperCase();
}

// Map a status keyword to a Pill colour tone.
export const STATUS_TONE = {
  deferred: 'amber', retry: 'blue', active: 'green', running: 'green',
  idle: 'neutral', error: 'red', ok: 'green', pending: 'amber',
  spam: 'amber', phishing: 'red', spoof: 'red', info: 'neutral',
  warn: 'amber', reject: 'red', missing: 'amber', disabled: 'neutral',
  inactive: 'neutral', verified: 'green', found: 'green',
};
export const tone = (k) => STATUS_TONE[k] || 'neutral';

// ---- Navigation ------------------------------------------------
export const NAV = [
  { key: 'dashboard', label: 'Dashboard' },
  { group: 'Mail' },
  { key: 'mailboxes', label: 'Mailboxes', badge: '128' },
  { key: 'domains', label: 'Domains' },
  { key: 'aliases', label: 'Aliases' },
  { group: 'Flow' },
  { key: 'queue', label: 'Queue', badge: '14' },
  { key: 'quarantine', label: 'Quarantine', badge: '5' },
  { key: 'spam', label: 'Spam filter' },
  { group: 'System' },
  { key: 'syncjobs', label: 'Sync jobs' },
  { key: 'logs', label: 'Logs' },
  { group: 'Apps' },
  { key: 'webmail', label: 'Webmail' },
];

// ---- Account ---------------------------------------------------
export const ACCOUNT = { name: 'Jamie Doe', role: 'Admin · Acme', email: 'jamie@acme.io', server: 'mail.acme.io' };

// ---- Dashboard: services & KPIs -------------------------------
export const SERVICES = [
  { name: 'postfix', meta: '18d', tone: 'green' },
  { name: 'mysql', meta: '18d', tone: 'green' },
  { name: 'dovecot', meta: '18d', tone: 'green' },
  { name: 'redis', meta: '18d', tone: 'green' },
  { name: 'rspamd', meta: '6d', tone: 'green' },
  { name: 'clamd', meta: 'high load', tone: 'amber' },
  { name: 'sogo', meta: '18d', tone: 'green' },
  { name: 'nginx', meta: '18d', tone: 'green' },
];

export const KPIS = [
  { label: 'Mailboxes', value: '128', delta: '▲ 3 new this week', tone: 'green' },
  { label: 'Domains', value: '6', delta: '2 pending DNS', tone: 'amber' },
  { label: 'Storage', value: '412 GB', pct: 41, note: '41% of 1 TB' },
  { label: 'In queue', value: '14', delta: '2 deferred', tone: 'amber', dot: true },
];

export const CHART = [52, 66, 60, 82, 74, 40, 90]; // % heights, last is peak
export const CHART_DAYS = ['W', 'T', 'F', 'S', 'S', 'M', 'T'];

// ---- Mailboxes -------------------------------------------------
// [name, local, domain, usedGB, maxGB, active, lastLogin]
const MB_RAW = [
  ['Jamie Doe', 'jamie', 'acme.io', 3.2, 10, true, '2m ago'],
  ['Ana Ruiz', 'a.ruiz', 'acme.io', 6.8, 10, true, '1h ago'],
  ['Billing', 'billing', 'acme.io', 0.4, 5, true, '3h ago'],
  ['Ops Team', 'ops', 'example.com', 4.1, 10, true, '12m ago'],
  ['Support', 'support', 'cow.farm', 2.3, 5, true, '5m ago'],
  ['Kai Park', 'k.park', 'acme.io', 8.9, 10, false, '6d ago'],
  ['Marketing', 'marketing', 'example.com', 1.7, 5, true, '40m ago'],
  ['Newsletter', 'newsletter', 'acme.io', 5.5, 10, true, '4m ago'],
  ['No-Reply', 'no-reply', 'cow.farm', 0.1, 2, true, '—'],
  ['Sync Backup', 'sync', 'mail.dev', 0.9, 20, true, '22m ago'],
];
export const MAILBOXES = MB_RAW.map(r => {
  const pct = Math.round(r[3] / r[4] * 100);
  return {
    name: r[0], local: r[1], domain: r[2], addr: r[1] + '@' + r[2],
    used: r[3], max: r[4], pct, quota: r[3] + ' / ' + r[4] + ' GB',
    active: r[5], last: r[6], initials: initials(r[0]),
  };
});

// list of all mailbox addresses (for recipient pickers)
export const MAILBOX_ADDRS = MAILBOXES.map(m => m.addr);

// ---- Domains ---------------------------------------------------
// [name, boxes, aliases, usedGB, maxGB, dkim, dns, active]
const DOM_RAW = [
  ['acme.io', 64, 12, 210, 512, 'active', 'ok', true],
  ['example.com', 32, 8, 88, 256, 'active', 'ok', true],
  ['cow.farm', 18, 5, 41, 128, 'active', 'ok', true],
  ['mail.dev', 8, 2, 12, 64, 'missing', 'pending', true],
  ['team.io', 4, 1, 9, 32, 'active', 'ok', true],
  ['hey.acme.io', 2, 0, 3, 16, 'missing', 'pending', false],
];
export const DOMAINS = DOM_RAW.map(r => ({
  name: r[0], boxes: r[1], aliases: r[2], used: r[3], max: r[4],
  pct: Math.round(r[3] / r[4] * 100), quota: r[3] + ' / ' + r[4] + ' GB',
  dkim: r[5], dns: r[6], active: r[7],
  dkimLabel: r[5] === 'active' ? 'active' : 'missing',
  dnsLabel: r[6] === 'ok' ? 'verified' : 'pending',
}));

// Sample DKIM / DNS detail for the first domain
export const DKIM = {
  name: 'acme.io', selector: 'dkim._domainkey', bits: '2048-bit RSA', created: 'created 12 Mar 2025',
  boxes: 64, aliases: 12, quota: '210 / 512 GB',
  dkimLabel: 'DKIM active', dnsLabel: 'DNS verified',
  key: 'v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC7Xy2m2r0kFq9nJ8vT3pQ2wZ1mN4hL0eR8yK6bD9cF7aX5sG3uH1oP2iJ4kM6nB8vC0dE2fA7gT9rW1qY3zU5xI8lO4pS6mQwIDAQAB',
  records: [
    { type: 'MX', host: '@', value: '10 mail.acme.io.', status: 'ok', label: 'found' },
    { type: 'A', host: 'mail', value: '203.0.113.24', status: 'ok', label: 'found' },
    { type: 'SPF', host: '@', value: 'v=spf1 mx ~all', status: 'ok', label: 'found' },
    { type: 'DKIM', host: 'dkim._domainkey', value: 'v=DKIM1; k=rsa; p=MIGfMA0…IDAQAB', status: 'ok', label: 'found' },
    { type: 'DMARC', host: '_dmarc', value: 'v=DMARC1; p=quarantine; rua=mailto:dmarc@acme.io', status: 'ok', label: 'found' },
  ],
};

// ---- Aliases ---------------------------------------------------
// [address, forwardsTo, active]
const ALI_RAW = [
  ['sales@acme.io', 'jamie@acme.io, a.ruiz@acme.io', true],
  ['info@acme.io', 'ops@example.com', true],
  ['support@cow.farm', 'support-team (4 mailboxes)', true],
  ['careers@acme.io', 'hr@acme.io', true],
  ['postmaster@acme.io', 'jamie@acme.io', true],
  ['old-marketing@example.com', 'marketing@example.com', false],
  ['press@team.io', 'jamie@acme.io', true],
  ['dmarc@acme.io', 'dmarc-reports@acme.io', true],
];
export const ALIASES = ALI_RAW.map(r => ({
  addr: r[0], goto: r[1], active: r[2],
  statusLabel: r[2] ? 'active' : 'inactive',
}));

// ---- Queue -----------------------------------------------------
// [sender, recipient, size, age, status]
const QUE_RAW = [
  ['newsletter@acme.io', '312 recipients', '2.1 MB', '4m', 'deferred'],
  ['billing@acme.io', 'j.ruiz@northwind.com', '84 KB', '11m', 'retry'],
  ['no-reply@cow.farm', 'ops@example.com', '12 KB', '1m', 'active'],
  ['sync@mail.dev', 'backup@acme.io', '640 KB', '22m', 'deferred'],
  ['alerts@acme.io', 'oncall@acme.io', '18 KB', '2m', 'active'],
  ['team.io', 'dmarc@google.com', '44 KB', '31m', 'retry'],
  ['marketing@example.com', 'list (1,204)', '5.6 MB', '48m', 'deferred'],
  ['support@cow.farm', 'maria@client.co', '9 KB', '8m', 'active'],
];
export const QUEUE = QUE_RAW.map(r => ({ sender: r[0], rcpt: r[1], size: r[2], age: r[3], status: r[4] }));

// ---- Quarantine -----------------------------------------------
// [subject, from, to, score, held, reason]
const QUAR_RAW = [
  ['Your invoice is overdue', 'accounts@0x-billing.ru', 'jamie@acme.io', 9.8, '2m ago', 'spam'],
  ['RE: shared document', 'no-reply@drive-share.co', 'ops@example.com', 7.2, '14m ago', 'phishing'],
  ['Win a $500 gift card', 'promo@deals-now.biz', 'marketing@example.com', 6.1, '38m ago', 'spam'],
  ['Verify your account now', 'security@paypa1.com', 'k.park@acme.io', 9.1, '1h ago', 'phishing'],
  ['Cheap meds online', 'pharma@rx-cheap.net', 'support@cow.farm', 8.4, '2h ago', 'spam'],
  ['Urgent: CEO request', 'ceo@acme-io.co', 'billing@acme.io', 8.9, '3h ago', 'spoof'],
  ['Your package is waiting', 'tracking@dhl-express.top', 'a.ruiz@acme.io', 5.7, '4h ago', 'spam'],
];
export const QUARANTINE = QUAR_RAW.map(r => ({
  subj: r[0], from: r[1], to: r[2], score: r[3].toFixed(1), when: r[4], reason: r[5],
  scoreTone: r[3] >= 8 ? 'red' : r[3] >= 6 ? 'amber' : 'muted',
}));

// ---- Logs ------------------------------------------------------
// [time, service, level, message]
const LOG_RAW = [
  ['09:42:11', 'rspamd', 'reject', 'spam rejected from 45.83.201.10 score=9.8'],
  ['09:41:58', 'postfix', 'info', 'delivered to ana@acme.io (12 KB)'],
  ['09:41:32', 'dovecot', 'info', 'imap login jamie@acme.io from 88.12.4.9'],
  ['09:40:15', 'rspamd', 'info', 'greylisted 185.220.101.7'],
  ['09:39:02', 'postfix', 'warn', 'deferred billing@acme.io retry in 5m'],
  ['09:38:44', 'sogo', 'info', 'calendar sync ok jamie@acme.io'],
  ['09:37:10', 'acme', 'info', 'certificate renewed *.acme.io (89d)'],
  ['09:36:51', 'clamd', 'warn', 'scan queue high, 128 items pending'],
  ['09:35:22', 'postfix', 'info', 'delivered to ops@example.com (4 KB)'],
  ['09:34:03', 'dovecot', 'error', 'auth failed k.park@acme.io (3 attempts)'],
  ['09:33:40', 'rspamd', 'reject', 'phishing rejected paypa1.com score=9.1'],
  ['09:32:12', 'nginx', 'info', 'GET /admin 200 jamie'],
  ['09:31:05', 'postfix', 'info', 'new mailbox support@cow.farm'],
  ['09:30:44', 'sogo', 'info', 'session start ops@example.com'],
];
export const LOGS = LOG_RAW.map(r => ({ time: r[0], svc: r[1], level: r[2], msg: r[3] }));
export const LOG_SERVICES = ['All', 'postfix', 'dovecot', 'rspamd', 'sogo'];

// ---- Sync jobs -------------------------------------------------
// [name, source, target, status, lastRun, every]
const SYNC_RAW = [
  ['Gmail import — Jamie', 'imap.gmail.com:993', 'jamie@acme.io', 'running', '2m ago', '15 min'],
  ['Legacy IMAP — Ops', 'mail.old-corp.com', 'ops@example.com', 'idle', '1h ago', '30 min'],
  ['Fastmail backup', 'imap.fastmail.com', 'backup@acme.io', 'idle', '22m ago', '60 min'],
  ['Yahoo — Kai', 'imap.mail.yahoo.com', 'k.park@acme.io', 'error', '3h ago', '15 min'],
  ['Outlook — Marketing', 'outlook.office365.com', 'marketing@example.com', 'running', '5m ago', '20 min'],
];
export const SYNCJOBS = SYNC_RAW.map(r => ({ name: r[0], src: r[1], target: r[2], status: r[3], last: r[4], every: r[5] }));

// ---- Spam ------------------------------------------------------
export const SPAM_KPIS = [
  { label: 'Scanned 24h', value: '42,318', tone: 'ink' },
  { label: 'Rejected', value: '2,317', tone: 'red' },
  { label: 'Greylisted', value: '1,044', tone: 'amber' },
  { label: 'Ham learned', value: '8,902', tone: 'green' },
];
export const ALLOWLIST = ['*.github.com', 'receipts@stripe.com', '*.acme.io'];
export const BLOCKLIST = ['0x-billing.ru', 'paypa1.com', '*.top'];
export const SPAM_RULES = [
  { title: 'Greylisting', desc: 'Defer unknown senders briefly to deter spam bots', on: true },
  { title: 'Reject spam over 10.0', desc: 'Bounce messages that score above the reject threshold', on: true },
  { title: 'Bayesian learning', desc: 'Learn from messages users mark as spam or ham', on: true },
  { title: 'SPF · DKIM · DMARC enforcement', desc: 'Penalise messages that fail authentication', on: true },
  { title: 'Rate limiting', desc: 'Throttle senders exceeding hourly message limits', on: false },
];

// ---- Webmail ---------------------------------------------------
export const FOLDERS = [
  { name: 'Inbox', count: 7, active: true },
  { name: 'Sent' },
  { name: 'Drafts', count: 2 },
  { name: 'Archive' },
  { name: 'Junk', count: 5 },
  { name: 'Trash' },
];
export const WEBMAIL_LABELS = [
  { name: 'Ops', color: 'var(--accent)' },
  { name: 'Billing', color: 'var(--green)' },
  { name: 'Urgent', color: 'var(--red)' },
  { name: 'Follow-up', color: 'var(--amber)' },
  { name: 'Personal', color: 'var(--blue)' },
];
// [from, subject, preview, time, unread, starred, fromAddr, body]
const EMAIL_RAW = [
  ['Ana Ruiz', 'Q3 roadmap review', 'Sharing the deck ahead of Thursday — let me know if the timeline still works for eng.', '9:24 AM', true, false, 'ana.ruiz@acme.io',
    'Hi Jamie,\n\nSharing the deck ahead of Thursday\u2019s review — it\u2019s in the shared drive under Q3 / Roadmap. The main open question is whether the migration work lands before or after the billing rework; eng thinks the timeline still holds either way.\n\nCould you skim the deliverability section (slides 8\u201311) before we meet?\n\nThanks,\nAna'],
  ['GitHub', '[acme/mailfold] PR #212 merged', 'Your pull request was merged into main by k-park.', '8:51 AM', true, true, 'notifications@github.com',
    'Your pull request #212 \u201CWarm dark theme + accent tokens\u201D was merged into main by k-park.\n\n14 files changed \u00B7 +842 \u2212310\n\nView the pull request on GitHub to see the full diff.'],
  ['Stripe', 'Your July invoice', 'Receipt for your subscription is attached. Total billed: $49.00.', '8:02 AM', false, false, 'receipts@stripe.com',
    'Thanks for your business!\n\nYour invoice for July 2026 is attached. Total billed: $49.00, charged to Visa ending 4242. Your next renewal is 1 August 2026.'],
  ['Kai Park', 'Re: DNS migration', 'Confirmed — DKIM records propagated overnight, deliverability is back to 99%.', 'Yesterday', false, true, 'k.park@acme.io',
    'Confirmed — the DKIM records propagated overnight and deliverability is back to 99.2% across all domains.\n\nI\u2019ll keep the old selector active for another 48 hours as a fallback, then rotate it out.\n\n— Kai'],
  ['Postmaster', 'DMARC aggregate report', '1,204 messages evaluated, 99.2% aligned, 3 sources failed.', 'Yesterday', false, false, 'postmaster@acme.io',
    'DMARC aggregate report for acme.io (24h window).\n\n1,204 messages evaluated \u00B7 99.2% aligned \u00B7 3 sources failed alignment.\n\nTop failing source: 45.83.201.10 (9 messages, all rejected).'],
  ['Maria Lopez', 'Lunch next week?', 'Are you around Tuesday or Wednesday? Would love to catch up.', 'Mon', false, false, 'maria@lopez.co',
    'Hey! Are you around Tuesday or Wednesday next week? Would love to catch up properly — it\u2019s been ages.\n\nMaria'],
  ['Fastmail', 'Backup completed', 'Your scheduled sync finished successfully — 8,412 messages.', 'Mon', false, false, 'sync@fastmail.com',
    'Your scheduled sync job finished successfully.\n\n8,412 messages copied \u00B7 0 errors \u00B7 4m 18s.\n\nThe next run is scheduled for tonight at 03:00.'],
];
export const EMAILS = EMAIL_RAW.map(r => ({
  from: r[0], subject: r[1], preview: r[2], time: r[3],
  unread: r[4], starred: r[5], fromAddr: r[6], body: r[7], initials: initials(r[0]),
}));

// Domain suffixes for address builders
export const DOMAIN_SUFFIXES = ['@acme.io', '@example.com', '@cow.farm', '@team.io'];
