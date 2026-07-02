# Mailfold frontend

The Mailfold web UI: a Vite + React single-page app that provides the admin
panel and the built-in webmail client, served by the Go backend.

## Stack

- React (JSX — no TypeScript) with Vite build tooling.
- A vendored design system under `src/ds/` (atoms / molecules / organisms +
  tokens/base/components CSS).
- A lightweight in-house i18n layer (`src/i18n/`): every user-facing string
  goes through `t()`; English ships today, and adding a language is a drop-in
  locale JSON.
- Talks to the Go backend under `/api/*` (bearer token in `localStorage`).

## Layout

- `src/main.jsx` → `src/App.jsx` (auth gate) → `src/app/Shell.jsx` (app shell +
  routing) and the pages under `src/pages/`.
- `src/api/client.js` — the REST client; `src/auth/` — the admin and webmail
  auth contexts.

## Develop

```bash
npm install
npm run dev     # Vite dev server; proxies /api and /dav to a live backend
```

The dev proxy target is configured in `vite.config.js` (env `VITE_PROXY`).

## Conventions

- All code and comments in English.
- The production build outputs to `frontend/dist/`, which the Go backend serves
  (see `MAILFOLD_FRONTEND_DIR`).
