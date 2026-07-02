# Mailfold Web UI — coverage gaps & build plan

_Generated from an automated scan of the 12-page design kit against the Go backend API._


## 1. Pages NOT in the design (do NOT build yet)

Backend features that have no screen in the supplied design. Grouped with where each could live.


### Contacts / Address Book (CardDAV)
- **Endpoints:** `GET /.well-known/carddav`, `PROPFIND/REPORT/GET/PUT/DELETE/OPTIONS /dav/carddav/`
- **Why:** The backend exposes a full CardDAV subtree (contacts sync over WebDAV/Basic-auth) that no design page covers. Admins/users need to see and manage address books, and — more realistically for a web UI — see the CardDAV URL + per-mailbox client-setup instructions (Apple Contacts, Thunderbird). The raw WebDAV verbs are machine-to-machine and won't be driven from React, but discovery/URL surfacing belongs somewhere.
- **Could nest under:** Webmail (a Contacts view/tab) for end-users; DAV setup URLs under Settings

### Calendar (CalDAV)
- **Endpoints:** `GET /.well-known/caldav`, `PROPFIND/REPORT/GET/PUT/DELETE/OPTIONS /dav/caldav/`
- **Why:** Same as CardDAV: a CalDAV subtree exists with no page. A real SOGo-style client would offer a calendar view; at minimum the CalDAV URL and setup instructions must be surfaced. DAV only mounts when cfg.DBPath is set, so the UI must gate on a capability probe.
- **Could nest under:** Webmail (Calendar tab) for end-users; DAV URLs under Settings

### App passwords
- **Endpoints:** `GET /api/app-passwords/{mailbox}`, `POST /api/app-passwords`, `DELETE /api/app-passwords`
- **Why:** Per-mailbox application passwords (IMAP/SMTP client creds bypassing 2FA) are a core mailbox-admin feature and a Settings/Security expectation. No page manages them today; the generated secret is shown once and must be surfaced.
- **Could nest under:** A per-mailbox detail Drawer under Mailboxes, and/or a Security section under Settings

### Sieve filters
- **Endpoints:** `GET /api/filters`, `POST /api/filters`, `PUT /api/filters`, `DELETE /api/filters`
- **Why:** Server-side mail rules (Sieve) per mailbox/domain — vacation/auto-reply, move-to-folder, forward. A common admin+user feature with full CRUD and no page. Needs a script editor or a rule-builder UI.
- **Could nest under:** A mailbox detail Drawer under Mailboxes, or a Webmail 'Filters' settings sub-view

### Temporary (time-limited) aliases
- **Endpoints:** `GET /api/temp-aliases/{mailbox}`, `POST /api/temp-aliases`
- **Why:** Throwaway/expiring aliases per mailbox — related to but distinct from permanent aliases (keyed by mailbox, carry a validity window). No page.
- **Could nest under:** Aliases (a second tab 'Temporary aliases') or a mailbox detail Drawer

### Domain admins
- **Endpoints:** `GET /api/domain-admins`, `POST /api/domain-admins`, `PUT /api/domain-admins`, `DELETE /api/domain-admins`
- **Why:** Delegated admins scoped to specific domains — an access-control feature with full CRUD and no page. Needed for multi-tenant delegation.
- **Could nest under:** Settings (an 'Admins & access' section) or a per-domain detail tab under Domains

### Super-admins (mailcow admins)
- **Endpoints:** `GET /api/admins`, `POST /api/admins`, `PUT /api/admins`, `DELETE /api/admins`
- **Why:** Full mailcow admin accounts (distinct from the single Mailfold login admin and from domain-admins). CRUD with no page; governs who can administer the whole server.
- **Could nest under:** Settings ('Admins & access' section)

### Resources (rooms / equipment)
- **Endpoints:** `GET /api/resources`, `POST /api/resources`, `PUT /api/resources`, `DELETE /api/resources`
- **Why:** Bookable calendar resources (meeting rooms, equipment) used by SOGo scheduling. Full CRUD, no page.
- **Could nest under:** Standalone (small), or a tab under Domains, or under Settings

### OAuth2 clients
- **Endpoints:** `GET /api/oauth2-clients`, `POST /api/oauth2-clients`, `DELETE /api/oauth2-clients`
- **Why:** OAuth2 client registrations for apps that authenticate against mailcow/SOGo. Security-sensitive (client secret shown once). No page.
- **Could nest under:** Settings ('Integrations' / 'API & OAuth' section)

### Native Mailfold API keys (NOT YET IN BACKEND)
- **Endpoints:** `(none — no /api/api-keys routes, handlers, or auth path exist today)`
- **Why:** The brief mentions an API-keys subsystem with no page, but the scan confirms it does NOT exist in the Go backend — the only key is the upstream mailcow X-API-Key held server-side and never exposed. Flagged so it is NOT built on the frontend until backend routes + a design page exist. Do not scaffold this.
- **Could nest under:** standalone (future); do not build now

### Forwarding hosts
- **Endpoints:** `GET /api/forwarding-hosts`, `POST /api/forwarding-hosts`, `DELETE /api/forwarding-hosts`
- **Why:** Trusted hosts allowed to forward mail (spam-scan bypass). Routing/security config with no page.
- **Could nest under:** Settings ('Routing & relay' section) or Spam (trusted senders)

### Transports
- **Endpoints:** `GET /api/transports`, `POST /api/transports`, `PUT /api/transports`, `DELETE /api/transports`
- **Why:** Postfix transport maps (per-destination delivery routing). Advanced routing config, full CRUD, no page.
- **Could nest under:** Settings ('Routing & relay' section)

### Relay hosts
- **Endpoints:** `GET /api/relayhosts`, `POST /api/relayhosts`, `PUT /api/relayhosts`, `DELETE /api/relayhosts`
- **Why:** Smarthost/relay definitions (send outbound via an upstream with credentials). Routing config, full CRUD, no page.
- **Could nest under:** Settings ('Routing & relay' section)

### TLS policy maps
- **Endpoints:** `GET /api/tls-policies`, `POST /api/tls-policies`, `DELETE /api/tls-policies`
- **Why:** Per-destination TLS enforcement policy (MTA-STS-like). Security/routing config, no page.
- **Could nest under:** Settings ('Routing & relay' or a 'Security' section)

### BCC maps
- **Endpoints:** `GET /api/bcc`, `POST /api/bcc`, `DELETE /api/bcc`
- **Why:** Silent/journaling BCC of a sender or recipient (compliance/audit). No page.
- **Could nest under:** Settings ('Routing & relay' section) or a per-domain tab under Domains

### Recipient maps
- **Endpoints:** `GET /api/recipient-maps`, `POST /api/recipient-maps`, `DELETE /api/recipient-maps`
- **Why:** Rewrite/redirect recipient addresses at the transport level. Routing config, no page.
- **Could nest under:** Settings ('Routing & relay' section)

### Domain templates
- **Endpoints:** `GET /api/domain-templates`, `POST /api/domain-templates`, `PUT /api/domain-templates`, `DELETE /api/domain-templates`
- **Why:** Reusable default settings applied when creating a domain (quota, limits, DKIM length). Feeds the Add-domain wizard defaults. No page.
- **Could nest under:** Settings ('Templates' section) with the create-domain wizard consuming them

### Mailbox templates
- **Endpoints:** `GET /api/mailbox-templates`, `POST /api/mailbox-templates`, `PUT /api/mailbox-templates`, `DELETE /api/mailbox-templates`
- **Why:** Reusable defaults for new mailboxes (quota, ACLs, TLS/spam policy). Feeds the Add-mailbox wizard. No page.
- **Could nest under:** Settings ('Templates' section) with the create-mailbox wizard consuming them

### Rspamd settings maps
- **Endpoints:** `GET /api/rspamd-settings`, `POST /api/rspamd-settings`, `DELETE /api/rspamd-settings`
- **Why:** Raw Rspamd settings maps (advanced scoring/rule scoping). The Spam page's rule toggles map onto these but the raw map CRUD has no dedicated surface.
- **Could nest under:** Spam (an 'Advanced / raw rules' section)

### Per-mailbox & per-domain rate limits
- **Endpoints:** `GET /api/ratelimits/mailbox`, `PUT /api/ratelimits/mailbox`, `GET /api/ratelimits/domain`, `PUT /api/ratelimits/domain`
- **Why:** Outbound send rate limits (msgs/hour) at mailbox and domain scope. The Spam page names 'rate limiting' but has no editor; edit-only endpoints exist. No page.
- **Could nest under:** Spam (rate-limit editor), plus per-domain under Domains detail and per-mailbox under Mailboxes detail

### Pushover notifications
- **Endpoints:** `PUT /api/pushover`
- **Why:** Per-mailbox Pushover push-notification config (edit-only). Maps to the missing Notifications panel. No page.
- **Could nest under:** Settings ('Notifications' panel — which is listed in SUBNAV but never rendered)

### Fail2ban configuration
- **Endpoints:** `GET /api/fail2ban`, `PUT /api/fail2ban`
- **Why:** Intrusion-prevention config (ban time, retries, whitelist/blacklist netblocks). Security feature with GET/PUT and no dedicated page; naturally sits beside Logs (which surface the ban events).
- **Could nest under:** Settings ('Security' section) or a tab on Logs

### Container status / version / storage (System)
- **Endpoints:** `GET /api/status/containers`, `GET /api/status/version`, `GET /api/status/vmail`
- **Why:** mailcow container health, version, and vmail storage. Dashboard consumes version+storage+health but there is no dedicated System page for full container-by-container status, restart hints, or the update flow implied by Settings' 'Update available' pill.
- **Could nest under:** Dashboard (health panel is the natural owner) with a fuller 'System' expansion under Settings/Server


## 2. What each supplied page needs to be fully functional


### Login.jsx
- Add useState for email/password and make both Inputs controlled (value+onChange); today they are uncontrolled and unreadable.
- Wrap fields in a real <form onSubmit> with e.preventDefault() so Enter submits; the Sign in button is not in a form and has no onClick.
- Wire Sign in to POST /api/auth/login {email,password}; on 200 store the returned bearer token (in memory + persisted, e.g. localStorage/sessionStorage) and navigate to Dashboard — no router/navigation exists on the page.
- Loading state: disable the button and show a spinner while the request is in flight (Button/Input have no disabled state wired).
- Error state: inline error slot/Toast for 401 invalid credentials, 429 rate-limit (surface Retry-After from the shared per-IP limiter), and network failure — no error markup exists.
- 2FA: the footer promises two-factor but there is no OTP step. Backend /api/auth/login has no 2FA step today, so either add a real TOTP verify step (needs backend) or remove the promise; do NOT ship a footnote the flow can't honor.
- SSO button: backend has no OIDC/SAML start route, so either hide the 'Single sign-on (SSO)' button behind a capability flag or leave it disabled — do not wire it to a non-existent endpoint.
- 'Forgot?' link: backend has no forgot-password endpoint; give it no dead href — hide it or point at admin docs until a reset flow exists backend-side.
- whoami bootstrap: on mount call GET /api/auth/me with any stored token and redirect an already-authenticated admin away from /login.
- Field validation: required + email-format checks with per-field messages before submit.
- Show/hide password toggle and optional remember-me are absent.
- Fix the asset: change the raw <img src='./assets/mailfold-mark.png'> (line 20) to an ESM import (import markUrl from '../assets/mailfold-mark.png') or move it to public/assets, and add an onError fallback; otherwise it 404s under Vite.
- Note the second (webmail) login: /api/webmail/login is a SEPARATE token namespace — decide whether admin Login and webmail login share this screen (they should not; webmail login belongs to the Webmail entry).

### Dashboard.jsx
- Replace all sample.js imports (KPIS, SERVICES, CHART/CHART_DAYS, QUEUE) with fetched state via a data hook; aggregate GET /api/status/version, GET /api/status/vmail, GET /api/status/containers, GET /api/mailboxes, GET /api/domains, GET /api/queue.
- No single 'overview' endpoint exists — KPIs must be composed client-side: mailbox count (from /api/mailboxes list/meta), domain count (/api/domains), storage (/api/status/vmail), queue depth (/api/queue length). Deltas ('▲3','2 pending DNS') have no backing data and must be dropped or computed from a stored prior snapshot.
- Service health must come from GET /api/status/containers, not the static 8-entry SERVICES; fix the hard-coded '8 of 9 healthy' literal (mismatched — derive N-of-M from the response) and per-service uptime/meta.
- Mail-volume chart: there is NO /api/stats/mail-volume endpoint in the backend; either drop the 7-day chart, derive a coarse proxy, or add a backend endpoint. Do not ship a fake chart.
- Queue preview: back QUEUE.slice(0,4) with GET /api/queue (needs a stable per-message id from the API to make rows actionable); the header '14' and '7,420 processed' literals must be derived or removed.
- Wire '+ New mailbox' to open the Wizard organism (mailbox create flow) → POST /api/mailboxes, then refetch KPIs.
- 'Export' button: no backend export endpoint — implement client-side CSV/JSON export of the fetched data or remove.
- 'Manage queue →' and queue rows: make them navigate to the Queue page (and/or open a Drawer); flush is POST /api/queue/flush (whole-queue only — there is NO per-message flush/delete/retry endpoint, so per-row actions cannot be offered here).
- Make KPI cards link to their sections (Domains→/domains, In queue→/queue).
- Wire AppShell chrome the page currently omits: pass onNavigate (router), onTheme (persist + set document.documentElement.dataset.theme), onSearch (open CommandPalette), and a logout handler → POST /api/auth/logout.
- Loading skeletons per panel, empty state for an empty queue, and per-panel error/retry — none exist.
- Auto-refresh/polling for live queue depth and container health, with a real timestamp instead of the hard-coded 'Tuesday · 2 July 2026' subtitle.

### Mailboxes.jsx
- Fetch rows from GET /api/mailboxes instead of the static 10-row MAILBOXES; reconcile the hard-coded '128 mailboxes across 6 domains' header with real counts from the list (and GET /api/domains for the domain count).
- Create flow: wire '+ New mailbox' to a Wizard/Drawer (local-part + domain Select from GET /api/domains, display name, quota GB, password + confirm/generate, active toggle) → POST /api/mailboxes; optionally seed defaults from GET /api/mailbox-templates.
- Row detail/edit Drawer opened by TableRow onClick (chevron implies it): show details + Edit/Reset-password/Delete → PUT /api/mailboxes / DELETE /api/mailboxes. This Drawer is also the natural home for the page-less per-mailbox features: app-passwords, Sieve filters, temp-aliases, per-mailbox rate limits.
- Delete confirmation via ConfirmModal(danger) → DELETE /api/mailboxes (kit's ConfirmModal is not imported here).
- Enable/disable: the active/disabled Pill is display-only; add a toggle → PUT /api/mailboxes with the active flag.
- FilterTabs (All/Active/Disabled): pass value+onSelect with state; today value='All' with no handler makes tabs inert.
- SearchInput: add value+onChange+state (client filter or debounced ?q=); today no value/onChange so typing does nothing.
- CSV import: 'Import CSV' needs a file picker + preview + upload — but there is NO backend bulk-import endpoint. Either add one backend-side or implement client-side row-by-row POST /api/mailboxes with progress; do not wire to a non-existent /import.
- Pagination: 128 mailboxes render flat with no pager — add page/limit UI; confirm whether GET /api/mailboxes supports server paging or page client-side.
- Sorting: column headers are static spans; add sort by name/quota%/last-login.
- Loading skeleton, empty state ('no mailboxes / no matches'), error banner, and success Toasts — all absent.
- Bulk actions: row checkboxes + select-all for bulk enable/disable/delete (mailcow edit/delete take arrays).

### Domains.jsx
- Fetch DOMAINS from GET /api/domains; DomainDetail is hard-wired to the module-level DKIM sample (acme.io) regardless of selection — parameterize it by the selected domain via GET /api/dkim/{domain}.
- Row navigation: wire TableRow onClick + the chevron to switch into detail mode / route to the domain; today the two modes are only reachable via the external `detail` prop.
- Back navigation: pass onBack to DomainDetail so '← Domains' returns to the list.
- Add-domain flow: wire '+ Add domain' to a Wizard (name, quota/max, mailbox+alias limits, active, DKIM key length) → POST /api/domains; seed defaults from GET /api/domain-templates.
- Edit-domain: 'Domain settings' opens nothing — add an edit Drawer/form → PUT /api/domains (quota, limits, active, relay). Also surface domain.active (in data, never shown) and per-domain rate limits (GET/PUT /api/ratelimits/domain), BCC, relayhost as sub-sections.
- Delete-domain: no control and no ConfirmModal — add a typed/danger confirm (destroys mailboxes/aliases) → DELETE /api/domains.
- DKIM rotate: 'Rotate key' needs a danger confirm (breaks signing until DNS propagates) → the backend has POST /api/dkim (generate) + DELETE /api/dkim (delete) but no single 'rotate' — implement rotate as delete-then-generate, then refetch GET /api/dkim/{domain}.
- Copy DKIM key: wire the 'Copy' button to navigator.clipboard + a copied Toast (currently inert).
- DNS verification: 'Verify DNS' / 'Check all DNS' have NO backing endpoint — mailcow has no native DNS check, so the Go backend must add a resolver-diff endpoint (MX/A/SPF/DKIM/DMARC expected-vs-found). Until it exists, disable these buttons; the DNS-records table and pending/verified pills otherwise have no live source.
- Data states: loading skeleton, empty state (no domains), error state — none exist.
- List search/filter/sort/pagination — none; add filter by DKIM/DNS status and active/disabled.
- Truncated DNS values (mf-truncate) with no way to see/copy the full value and no expected-vs-found diff on failure (all sample records are 'ok').
- Wire AppShell onNavigate/onTheme/onSearch (page passes none of its own).

### Aliases.jsx
- Fetch from GET /api/aliases instead of static ALIASES; derive the subtitle count (hard-coded '28 forwarding rules across 6 domains' contradicts the 8 rows).
- goto is a structured recipient LIST in mailcow but a free-text string in the sample — the create/edit form needs a structured goto[] (MultiSelect recipient picker from GET /api/mailboxes + GET /api/aliases), plus mailcow's special targets (learn-as-spam/ham, silent discard).
- Create: wire '+ New alias' to a Wizard/Drawer (local-part + domain Select from GET /api/domains, goto[] MultiSelect, active toggle, private_comment) → POST /api/aliases.
- Edit/detail Drawer: give TableRow onClick (chevron implies it) → prefill from the list/GET, edit targets/rename/toggle/comment → PUT /api/aliases. No AliasDetail equivalent exists (contrast DomainDetail).
- Delete + ConfirmModal(danger) → DELETE /api/aliases (no delete control anywhere today).
- Inline enable/disable: active is display-only via the Pill; add a per-row toggle → PUT /api/aliases with active flag.
- Search + FilterTabs (All/Active/Inactive) — both absent (Mailboxes has them); add to narrow 28+ rows. Add a Domain column/filter since the subtitle groups by domain.
- Pagination/virtualization — none (kit has no Pagination component; page client-side or add controls).
- Render goto as multiple Tokens/Pills instead of one truncated string; expand labels like 'support-team (4 mailboxes)' into resolvable members.
- Surface mailcow fields ignored today: private_comment, sogo_visible, created/modified, domain.
- Loading/empty/error states and success Toasts — none.
- Validation: address uniqueness, valid email/domain, ≥1 target, no self-referential loop.
- Wire AppShell onNavigate/onTheme/onSearch.

### Queue.jsx
- Fetch from GET /api/queue; QUEUE is static and rows lack a queue_id and real byte-size/timestamp — the API must return a stable id + numeric size + arrival time so Size/Age can be formatted and sorted.
- Header summary ('14 messages · 2 deferred · oldest 48m') is a hard-coded string that contradicts the 8 rows (3 deferred) — compute it from the fetched queue (no summary endpoint exists).
- 'Flush all': wire onClick → POST /api/queue/flush, ideally behind ConfirmModal, with a success/failure Toast (currently inert).
- 'Delete all': this is INERT AND unguarded — but note the backend has NO delete endpoint for the queue (only GET /api/queue and POST /api/queue/flush exist). There is no per-message flush/delete/hold/unhold and no delete-all route. So: either remove the 'Delete all' button and all per-row action affordances, or add the missing backend routes first. Do not wire buttons to non-existent endpoints.
- Because only flush-all is supported today, drop the implied per-row controls (view/flush/hold/unhold/delete) and the message-detail Drawer until GET /api/queue/{id} + mutation routes exist.
- Search/filter (by status/sender/recipient) and sorting (Age/Size) — none; and current values are unsortable display strings.
- Pagination and auto-refresh/poll — a real queue is large and volatile; add polling + paging.
- Loading skeleton, empty state ('Queue is empty' — the common healthy case), and error/retry — none exist.
- Wire AppShell onNavigate/onSearch/onTheme (dead on this page).

### Quarantine.jsx
- Fetch from GET /api/quarantine; QUARANTINE is static, rows are keyed by array index i (no stable id) which breaks targeted release/delete — the API must return a per-item id.
- Real selection: atoms/Checkbox is a decorative <span>, not an <input> — replace with a controlled checkbox (checked/onChange), add a header select-all (COLS[0] is empty), track selected ids in state, and enable/disable the bulk buttons by selection count.
- 'Delete': backend has DELETE /api/quarantine (delete items) — wire it behind ConfirmModal(danger); no confirm exists today and ConfirmModal is not imported.
- 'Release selected': the backend scan shows NO release endpoint (only GET list + DELETE). mailcow supports release via edit/quarantine, but Mailfold has not exposed it — either add POST /api/quarantine/release backend-side or disable the 'Release selected' button. Same for whitelist/blacklist-sender and raw .eml download (no routes today).
- Per-message detail Drawer (full headers, body/HTML preview, rspamd symbol breakdown) needs GET /api/quarantine/{id} which doesn't exist yet — gate the row-click Drawer on that route.
- Reconcile counts: NAV badge '5' vs 7 rows vs the hard-coded '7 messages held · auto-purge in 14 days' subtitle — derive count from the API; the 14-day retention needs a settings endpoint (none exists) so treat as static text or add one.
- Search/filters (by reason spam/phishing/spoof, score range, recipient/domain, date-held) and sortable columns (Score/Held) — none; headers are plain spans.
- Loading/empty ('No messages in quarantine')/error states — none.
- Post-action optimistic removal/refetch + success/error Toasts (Toast/Toaster unused).
- Pagination — Table renders every row.
- Wire AppShell onSearch/onNavigate/onTheme.

### Spam.jsx
- No React state at all — thresholds, rules, and lists are static constants (and threshold values 6.0/10.0 live only as inline JSX literals). Introduce useState/useReducer seeded from GETs plus a dirty flag so 'Save changes'/'Reset' become meaningful.
- There is NO /api/spam/* endpoint — Spam must aggregate page-less resources: GET /api/policy/allow/{domain} + GET /api/policy/deny/{domain} for allow/block lists, POST/DELETE /api/policy to add/remove entries, GET/POST/DELETE /api/rspamd-settings for rule maps, and GET/PUT /api/ratelimits/{domain,mailbox} for rate limiting. There is no stats endpoint, so the 24h KPIs (scanned/rejected/greylisted/ham) have no source — drop them or add a backend stats route.
- Allow/block lists are per-DOMAIN in the backend ({domain} path param) but the UI presents a single global list — add a domain scope selector; entries need an id for delete.
- '+ Add' (allow/block): wire to an inline form/Drawer with pattern validation (address vs wildcard) + duplicate detection → POST /api/policy; optimistic append.
- Chip removal: Chips have no × — add a deletable Chip variant + ConfirmModal → DELETE /api/policy.
- Rule toggles: ToggleRow's Toggle has no onChange — add onToggle(key) to flip 'on' + mark dirty; map each rule onto rspamd-settings/policy where a real backing exists (greylist/reject-over-10/Bayes/SPF-DKIM-DMARC enforcement/rate-limit). Rules with no backend mapping must be removed, not faked.
- Threshold slider: the two handles (40%/66.7%) are static — either build real draggable/keyboard-accessible handles bound to numeric add-header/reject values (validate addHeader<reject≤max) persisted via rspamd-settings, or replace with numeric inputs. Confirm the backend can actually persist thresholds (rspamd-settings map) before wiring.
- Confirmations for disabling a security rule (SPF/DKIM/DMARC, reject-over-10) and for 'Reset' (discard unsaved) via ConfirmModal.
- Loading skeleton, empty-list states, and error/success Toasts (Toast/Toaster unused) — none.
- Time-window selector for KPIs (label says 24h, no control); 'ink' tone silently drops to undefined.
- Wire AppShell onNavigate/onSearch/onTheme.

### SyncJobs.jsx
- Fetch from GET /api/syncjobs instead of the static 5-row SYNCJOBS; derive the subtitle ('5 IMAP/POP imports' is hard-coded).
- Create: wire '+ New sync job' to the Wizard with sync-specific fields that don't exist anywhere yet — source host + port + encryption (SSL/TLS/STARTTLS/none), username, password, target mailbox (Select from GET /api/mailboxes), poll interval, and mailcow options (subfolder2, maxage, maxbytes, delete2duplicates, delete1/delete2, automap, skipcrossduplicates, exclude regex, custom params, active) → POST /api/syncjobs.
- Edit: no edit form/affordance — add one prefilled from the row (backend has no GET /api/syncjobs/{id}; the list GET must return full config, or a detail route must be added) → PUT /api/syncjobs.
- Delete: no control/confirm — add per-row Delete → ConfirmModal(danger) → DELETE /api/syncjobs.
- Run-now / Pause-Resume: the two runtime actions are entirely absent — but the backend has NO /run or /pause route; pause/resume can be done via PUT /api/syncjobs with active 0/1, while 'run now' has no endpoint (disable it or add one). Do not wire /run.
- Detail Drawer: chevron implies a detail panel but there's no onClick/Drawer; a live sync log (GET /api/syncjobs/{id}/log) has no backend route — omit the log view until it exists, or surface the last error inline.
- Row onClick for selection/detail — TableRow supports it, none passed.
- Search (name/target) + FilterTabs (running/idle/error) — none; both molecules are controlled and need value+onSelect/onChange state.
- Status/last-run are static strings — add polling so 'running' jobs and 'error' status update; surface why an 'error' job failed.
- Loading/empty ('No sync jobs yet' + CTA)/error states and success Toasts — none.
- Show missing columns/fields: port, encryption/TLS mode, source username, active flag, last-run result.
- Wire AppShell onNavigate/onSearch/onTheme.

### Logs.jsx
- Fetch from GET /api/logs/{service} (count clamped 1..1000) instead of the static 14-row LOGS; the service must be one of the backend's allowlisted mailcow services.
- Service filter chips are raw <span> with 'All' hard-coded active via s==='All' and no onClick — replace with a controlled FilterTabs (selected state + onSelect) that refetches per service. Derive the service list from the server (or GET-per-service); the sample even contains nginx/clamd/acme not in LOG_SERVICES.
- 'Live' indicator is decorative: the backend has NO streaming/SSE/WebSocket endpoint — implement periodic polling of GET /api/logs/{service} with a real Pause/Resume + auto-scroll, and drop the 'Live' promise or relabel it 'auto-refresh'. Do not wire a non-existent stream.
- 'Download' button is inert and there is NO export endpoint — implement client-side download of the currently fetched/ filtered lines (txt/json/csv), or add a backend export route.
- No level filter (info/warn/error/reject), no free-text search/grep, no time-range picker — the backend only takes {service} + count, so level/search/range must be client-side over fetched lines (or backend params added).
- Pagination / 'load older': only a clamped count is supported (no cursor) — implement count-based 'load more' up to 1000.
- Row detail: messages are truncated (mf-truncate) and unrecoverable — add a click-to-expand (Drawer) showing the full line/structured fields; no per-line detail endpoint exists so keep it client-side from the fetched row.
- Loading skeleton, empty ('No log entries match this filter'), and error/reconnect states — none.
- Consider surfacing fail2ban (GET/PUT /api/fail2ban) here as a sibling tab since ban events show up in logs.
- Wire AppShell onNavigate/onTheme/onSearch.

### Settings.jsx
- PageHeader is imported but UNUSED (page renders a raw h1) — either use it or drop the import.
- Profile fields are hard-coded literals ('JD','Jamie Doe','jamie@acme.io','Administrator','Europe/Madrid') and ignore the ACCOUNT object the page already imports — bind to GET /api/auth/me (the only identity endpoint; there is NO /api/account/profile) and controlled state. Note: /api/auth/me returns only {user, expires_at}, so display-name/timezone/avatar have no backend source — those fields must be dropped or backend endpoints added.
- Make it a real form: convert every Input/Select/Segmented/Toggle from defaultValue/fixed value to controlled (value+onChange) with local state, and add a Save/Cancel action bar per section (no persistence affordance exists).
- Sub-nav (SUBNAV) items are inert with the first hard-coded active — add active state + onClick to switch sections or scroll-spy.
- Notifications panel is listed in SUBNAV but NO card renders — build it; the only backend hook is PUT /api/pushover (edit-only Pushover), so scope the panel to what the backend supports.
- Security section: 'Change' password, 'Sign out all', and the '2FA Enabled' pill have NO backend endpoints (no password-change, no sessions list, no 2FA routes exist) — either remove these controls or add backend routes first. 'Sign out all' at most maps to POST /api/auth/logout for the current token.
- Appearance: Theme Segmented reflects the `theme` prop but has no onSelect so it can't change theme — wire onSelect/onTheme to set document.documentElement.dataset.theme and persist; wire Accent swatches (set dataset.accent — note accent only applies when data-theme is on the same node) and Density. These are pure client-side prefs (no backend prefs endpoint exists) — persist to localStorage.
- Language & region Selects are the presentational Select (never opens) — swap for a controlled select or native <select>; persist to localStorage (no backend i18n prefs endpoint).
- Server card: hostname/version/update/backup are literals — source hostname+version from GET /api/status/version and storage from GET /api/status/vmail; there is NO update-apply or backup endpoint, so the 'Update available' / 'Run backup' actions must be removed or gated as informational.
- This page is the natural home for the routing/relay/templates/admins/fail2ban page-less resources (transports, relayhosts, tls-policies, bcc, recipient-maps, domain/mailbox-templates, admins, domain-admins, fail2ban, oauth2-clients, forwarding-hosts) as additional sub-sections — plan the sub-nav accordingly.
- Change-photo file input + upload has no backend route — remove or add one.
- Loading/error/success states, inline validation (email format, password strength/match), and unsaved-changes guard — none exist.

### Webmail.jsx
- This page uses the SEPARATE webmail token system, not the admin session — it needs its own POST /api/webmail/login gate (mailbox email+password → webmail bearer token; 503 when IMAP unconfigured) before rendering, and POST /api/webmail/logout. The admin Login page does not cover this.
- Fork or lift state into the Webmail organism: it takes a fixed selected=0 with no onSelect and renders MailListItem without onClick — add controlled selected state + row onClick to open messages (GET /api/webmail/message by UID for the reading pane; sample bodies are truncated demo strings).
- Folders: FolderItem has no onClick and the Work/Clients/Invoices/Newsletters trees are hard-coded in the organism — make folders data-driven from GET /api/webmail/folders with active-folder state; '+ New label'/'+ folder' → POST /api/webmail/folders.
- Message list: back it with GET /api/webmail/messages?folder=; wire the compact SearchInput (value+onChange) to GET /api/webmail/search; add pagination/infinite scroll (list renders all EMAILS at once) and live unread counts (the '7 unread' and per-folder counts are static and won't decrement on open).
- Compose: 'Compose' button is not wired to ComposeModal, and ComposeModal itself is inert (Subject/body/Send/Discard/'Cc·Bcc' toggle unwired, recipients display-only) — add open/close state, controlled fields, recipient MultiSelect (autocomplete from GET /api/mailboxes / a contacts source), Send → POST /api/webmail/send. Draft autosave has no backend route — drop the 'Draft saved' text or add one.
- Reply/Forward: buttons and the inline reply box are non-functional — build a reply composer (quoting) → POST /api/webmail/send. There are no dedicated reply/forward endpoints; compose via /send with In-Reply-To.
- Message actions: star/flag → POST /api/webmail/flag; archive/trash → POST /api/webmail/move (to Archive/Trash folder); permanent delete → POST /api/webmail/delete — all toolbar/per-row controls are display-only today; add handlers + optimistic update. Emptying trash / permanent delete needs a ConfirmModal (not imported here).
- Mark-as-read on open → POST /api/webmail/flag (\Seen); update unread accounting.
- Attachments: reading pane has no attachment list/download — wire GET /api/webmail/attachment by index; add HTML body sanitisation, To/Cc/Bcc display, and thread grouping (all absent).
- Bulk selection: select-all + per-row checkboxes are inert (Checkbox is visual) — add multi-select + bulk archive/move/delete/mark-read.
- Loading skeletons (folder tree, list, reader body), empty states ('no messages' / 'select a message' / 'no search results'), and error/send-failed/offline states — none.
- Contacts/Calendar: this is the natural host for the page-less CardDAV/CalDAV surfaces (contacts autocomplete, calendar view, or at least DAV setup URLs).
- Wire AppShell onNavigate/onTheme/onSearch/logout/notifications (all unpassed).


## 3. App-wide (cross-cutting) gaps

- Auth & session core: build an AuthContext/provider holding the admin bearer token from POST /api/auth/login, bootstrap via GET /api/auth/me on load, and clear on POST /api/auth/logout. Sessions are in-memory server-side (lost on restart, single-node) and stateless bearer (no cookies) — so persist the token client-side (sessionStorage/localStorage), attach Authorization: Bearer on every /api/* call, and on any 401 clear the session and route to /login. There is no refresh token and no cookie/CSRF, so no CSRF plumbing is needed (bearer-only).
- Second, DISTINCT webmail session: /api/webmail/* uses a separate, non-interchangeable token namespace with its own login/logout and TTL, returning 503 when IMAP is unconfigured. Model a second WebmailAuthContext scoped to the Webmail route; do not mix the two tokens. Probe capability (503) and disable Webmail if unconfigured.
- Routing shell: the kit has NO router. Add react-router (or equivalent). Public route /login (renders bare Login, the only page without mf-app chrome); a ProtectedRoute wrapper that redirects to /login without a valid session and renders AppShell + page for /dashboard, /mailboxes, /domains, /domains/:domain, /aliases, /queue, /quarantine, /spam, /syncjobs, /logs, /settings, /webmail. Thread onNavigate (router.push), current (active key), onTheme, onSearch (open CommandPalette) through AppShell for every page — today pages forward only {...props} and set none of these.
- Single API client module: centralize base URL, bearer injection (admin vs webmail), JSON parse, error normalization (map 400/401/403/429/500 to typed errors; surface 429 Retry-After from the shared per-IP login limiter), request-id capture (X-Request-Id header the backend sets), and the MaxBytesReader body cap. All mutation endpoints are collection-level with method-as-verb (GET/POST/PUT/DELETE on /api/mailboxes etc.) and mailcow-style array payloads — encode that convention once.
- Data-fetching layer: adopt a query/cache hook (TanStack Query or a small custom useFetch) providing loading/empty/error/refetch uniformly — every one of the 12 pages currently has zero states. Standardize skeletons, empty states, and error/retry banners as shared components (none exist in the kit).
- Global Toaster + toast queue: the kit ships Toast/Toaster with NO queue/auto-dismiss logic — build a ToastProvider (add/dismiss/auto-timeout) mounted once at app root; wire success/error feedback for all create/edit/delete/flush/release actions (no page wires feedback today).
- Modal/overlay conventions: Drawer/ConfirmModal/ComposeModal/Wizard/CommandPalette have NO isOpen prop — mount conditionally from page state. onClose fires on backdrop click; Esc-to-close is NOT built in (add a shared useEscapeKey/focus-trap). Standardize destructive actions behind ConfirmModal(danger).
- Presentational components that MUST be lifted/forked before use (kit note): Select (no onChange — add open state + option onClick or swap native), MultiSelect (only onRemove wired; text input + suggestions need a controlled key handler), Toggle & Checkbox (no onChange — attach onClick/flip parent bool; Quarantine/Webmail checkboxes are decorative <span>s), Webmail organism (rows/toolbar/folders static), ComposeModal (Subject/body/Send unwired), DomainDetail (all buttons except onBack unwired), threshold sliders in Spam. Plan the fork/extension work explicitly.
- Theming: theme+accent are pure CSS data-attributes (data-theme, data-accent) with NO JS logic. Hold {theme,accent} in a ThemeContext, set document.documentElement.dataset.theme/.accent, and persist to localStorage. Critical gotcha: data-accent only takes effect when data-theme is on the SAME node — so always set both on <html>. Wire Sidebar onTheme and Settings Appearance controls to this context.
- Backend-vs-design reality gaps to resolve up front (do NOT wire UI to endpoints that don't exist): NO endpoints for — dashboard mail-volume stats, DNS verification/check, queue per-message ops or delete-all (only flush-all), quarantine RELEASE/whitelist/raw-download, syncjob run/pause/log, log streaming/export, password change, active-sessions, 2FA, SSO/OIDC, forgot-password, account profile/avatar, mailbox CSV import, backups/update-apply. For each, either add the backend route, implement client-side, or hide/disable the control. Ship no dead buttons.
- 2FA/SSO honesty: the Login footer and Settings pill promise 2FA, and Login shows an SSO button, but the backend has none — either implement server-side or remove the copy/controls so the UI doesn't claim capabilities it lacks.
- Optimistic updates + rollback for mutations (toggle active, star/flag, release/delete), with refetch-on-settle; none exist and the kit is fully presentational.
- Real vs hard-coded literals: every page hard-codes counts/summaries that contradict their own sample rows ('128 mailboxes', '28 rules'/8 rows, '8 of 9 healthy'/8 services, queue '14'/8 rows, quarantine '7'/badge '5', '7,420 processed', '7 unread', dates). Route ALL such copy through fetched data.
- i18n/l10n: Settings offers language/region/timezone/time-format/first-day controls with no i18n framework and no backend prefs endpoint — either wire a client-side i18n lib + localStorage or descope these controls.
- Responsive/mobile: the shell is a fixed 240px sidebar + desktop grid tables with inline gridTemplateColumns; there's no mobile breakpoint, no collapsible sidebar, no responsive table strategy — add if mobile is in scope.
- Accessibility: presentational Toggle/Checkbox set roles but aren't real inputs; overlays lack focus trapping and Esc; tables use div-grid not semantic tables. Add keyboard/focus/ARIA passes.
- Assets & fonts before shipping: fix Login's raw <img> string (ESM import or public/) so Vite fingerprints it; decide embed-vs-CDN for the Google-Fonts @import in tokens.css (offline/CSP falls back to Georgia/mono, degrading serif-heavy headings) — self-host Newsreader+Geist Mono to be safe.
- Delete preview chrome on build: drop the in-browser Babel/unpkg runner, inline catalog CSS, and loading <div> from index.html; do not ship preview/app.jsx — write a real Vite entry that mounts the routed app and sets initial data-theme/data-accent on <html>.
- Config/env: API base URL, webmail-enabled flag, CORS (backend has a configurable allow-list) — provide via Vite env; the SPA is served by the Go fileserver catch-all in prod, so ensure the router uses history mode compatible with the index.html fallback.
- Fail2ban/security & page-less resource ownership: decide final IA — which of the ~24 page-less admin resources fold into Settings sub-tabs, Mailbox/Domain detail Drawers, Spam, or Webmail — before building, so navigation and the NAV model are extended coherently rather than per-page.


## 4. Build plan

1. 1. Scaffold Vite + React: `npm create vite@latest mailfold-web -- --template react`; add @vitejs/plugin-react, react-router-dom, and a data layer (@tanstack/react-query recommended). Configure vite.config.js with a dev proxy to the Go backend (/api and /dav → backend origin) and history-fallback so client routes resolve.
2. 2. Port the design system UNCHANGED: copy styles/ (tokens.css, base.css, components.css), components/ (atoms+molecules+organisms), lib/cx.js, data/sample.js, and assets/mailfold-mark.png into src/. All imports use explicit .js/.jsx extensions and named exports — no rewriting needed. Decide fonts (self-host Newsreader+Geist Mono via @font-face or keep the Google @import).
3. 3. Write the real entry src/main.jsx: import tokens.css → base.css → components.css IN THAT ORDER (load-bearing), then createRoot().render(<App/>). Keep <html lang=en data-theme=light data-accent=ochre> in index.html and strip ALL preview chrome (unpkg/Babel scripts, inline catalog CSS, loading div). Do not port preview/app.jsx.
4. 4. Build ThemeContext: hold {theme,accent}, set document.documentElement.dataset.theme/.accent (both on <html> — accent needs theme on the same node), persist to localStorage; expose onTheme/onAccent for Sidebar + Settings.
5. 5. Build the API client (src/api/client.js): base URL from env, bearer injection, JSON handling, typed error normalization (401→clear session, 429→Retry-After), X-Request-Id capture, and the collection-level GET/POST/PUT/DELETE + mailcow-array conventions. Add a distinct webmail client variant for the second token.
6. 6. Build AuthContext + ProtectedRoute: login via POST /api/auth/login, bootstrap via GET /api/auth/me, logout via POST /api/auth/logout, persist token; ProtectedRoute redirects to /login on missing/401. Add a separate WebmailAuthContext for the Webmail route (POST /api/webmail/login, 503 capability probe).
7. 7. Build the router shell (src/App.jsx): public /login (bare, no mf-app); protected routes wrapping AppShell for all 12 pages. Create a shared useAppShellProps hook that supplies current, onNavigate (router push), onTheme, onSearch (open CommandPalette) so every page threads real chrome instead of {...props}.
8. 8. Build shared infra components: ToastProvider + queue/auto-dismiss around Toaster/Toast; a mountable ConfirmModal helper (useConfirm); Skeleton, EmptyState, ErrorState, and a useQuery-based data hook with loading/empty/error/refetch; useEscapeKey + focus-trap for overlays.
9. 9. Fork/extend the presentational components that block wiring: a controlled Select (open state + option onClick or native swap), MultiSelect with a controlled input+key handler, functional Toggle/Checkbox (real inputs), a Webmail fork that accepts onSelect/onFolderSelect/onAction and per-row onClick, an extended ComposeModal (controlled Subject/body/recipients/Send), and an extended DomainDetail (Copy/Verify/Rotate/Settings handlers).
10. 10. Login page: controlled inputs, <form onSubmit>, POST /api/auth/login → store token → navigate to /dashboard; loading/error states; whoami redirect on mount; fix the mailfold-mark <img> to an ESM import; remove/disable the 2FA/SSO/Forgot affordances the backend can't honor.
11. 11. Dashboard: compose KPIs from /api/status/version + /api/status/vmail + /api/status/containers + /api/mailboxes + /api/domains + /api/queue; wire '+ New mailbox' Wizard → POST /api/mailboxes; make cards/queue link to pages; drop the mail-volume chart (no endpoint) or gate it; per-panel loading/empty/error + polling.
12. 12. Mailboxes: list from GET /api/mailboxes; create Wizard → POST; row Drawer (edit/reset-pw/delete via PUT/DELETE, plus app-passwords/filters/temp-aliases/ratelimits sub-sections); FilterTabs+SearchInput controlled; pagination/sort; ConfirmModal deletes; Toasts; states. Descope CSV import until a backend route exists.
13. 13. Domains: list from GET /api/domains; row onClick → detail route /domains/:domain parameterizing DomainDetail with GET /api/dkim/{domain}; add-domain Wizard → POST; edit Drawer → PUT; delete → DELETE (danger confirm); DKIM rotate = DELETE+POST /api/dkim; Copy→clipboard+Toast; disable Verify/Check-DNS until a backend DNS-diff route exists; states.
14. 14. Aliases: list from GET /api/aliases; create/edit Drawer with structured goto[] MultiSelect (from /api/mailboxes+/api/domains) → POST/PUT; delete → DELETE (confirm); add Search+FilterTabs+Domain column; render goto as tokens; states+Toasts.
15. 15. Queue: list from GET /api/queue; compute header summary from data; wire 'Flush all' → POST /api/queue/flush (confirm+Toast); REMOVE 'Delete all' and per-row actions (no backend routes); polling; empty/loading/error states.
16. 16. Quarantine: list from GET /api/quarantine with stable ids; real controlled checkboxes + select-all + selection state; 'Delete' → DELETE /api/quarantine behind ConfirmModal; DISABLE 'Release selected' and detail Drawer until backend release/detail routes exist; filters/sort/pagination; states.
17. 17. Spam: aggregate GET /api/policy/allow|deny/{domain} (+ domain scope selector), POST/DELETE /api/policy for entries, GET/POST/DELETE /api/rspamd-settings for rules, GET/PUT /api/ratelimits/{domain,mailbox}; controlled toggles+dirty flag; add/remove list entries with confirm; replace/gate the KPI cards (no stats endpoint) and threshold slider (numeric inputs unless rspamd-settings can persist); states+Toasts.
18. 18. SyncJobs: list from GET /api/syncjobs; create/edit Wizard with full IMAP/POP fields → POST/PUT; delete → DELETE (confirm); pause/resume via PUT active 0/1; disable run-now + log view (no routes); Search+FilterTabs; polling for running/error status; states.
19. 19. Logs: fetch GET /api/logs/{service} (count 1..1000); controlled FilterTabs per service (derive list server-side); replace 'Live' with polling + Pause/Resume + auto-scroll; client-side level filter + text search + 'load more'; client-side Download of fetched lines; expand-row Drawer; consider a fail2ban tab (GET/PUT /api/fail2ban); states.
20. 20. Settings: use PageHeader; bind identity to GET /api/auth/me (drop fields with no backend source); make sub-nav switch sections; wire Appearance (theme/accent/density) to ThemeContext + localStorage; controlled selects for language/region/tz (localStorage); source Server card from status endpoints; build Notifications panel scoped to PUT /api/pushover; remove/disable password-change, sessions, 2FA, backups, update-apply, change-photo (no backend); fold in the routing/relay/templates/admins/fail2ban page-less resources as sub-sections; validation + Save/Cancel + Toasts.
21. 21. Webmail: gate behind WebmailAuthContext (POST /api/webmail/login, 503 capability); data-driven folders (GET /api/webmail/folders, POST to create); message list (GET /api/webmail/messages) + search (GET /api/webmail/search) + pagination; reading pane (GET /api/webmail/message); controlled selection+active folder (forked organism); compose/reply/forward → POST /api/webmail/send with extended ComposeModal; flag → /flag, archive/trash → /move, delete → /delete (confirm); attachments via /api/webmail/attachment; mark-read on open; bulk selection; states; optionally surface Contacts/Calendar (CardDAV/CalDAV) + DAV setup URLs.
22. 22. Finalize: extend the NAV model for any new Settings sub-sections/detail routes; a11y pass (focus trap, Esc, ARIA, keyboard) on overlays and toggles; responsive/mobile pass if in scope; production build check that the Go SPA catch-all serves index.html with history-mode routing; verify fonts/assets fingerprint correctly; audit that NO control is wired to a non-existent endpoint (every disabled/removed action documented).
