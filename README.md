# Mailfold

A modern web frontend (admin panel + webmail) for [mailcow-dockerized](https://github.com/mailcow/mailcow-dockerized).

**Mailfold** — the name plays on two meanings: a *fold* is both the crease of a letter (mail) and an enclosure for livestock, nodding to mailcow's cattle branding and the "herd" of domains and mailboxes the panel manages.

## Stack

- **Backend:** Go — a thin, typed API layer on top of the mailcow REST API.
- **Frontend:** React — UI is delivered from a separate design project (WIP).

All code and documentation are written in English.

## Architecture

```
React SPA  ──▶  Mailfold Go backend  ──▶  mailcow API (/api/v1/...)
```

The Go backend authenticates to mailcow with an API key, exposes a clean REST
surface to the frontend, and serves the built SPA.

## Backend

### Configuration

The backend is configured entirely through environment variables (see
[`.env.example`](.env.example)):

| Variable | Default | Description |
| --- | --- | --- |
| `MAILFOLD_ADDR` | `:8080` | Listen address. |
| `MAILFOLD_MAILCOW_URL` | — (required) | Base URL of the mailcow instance. |
| `MAILFOLD_MAILCOW_API_KEY` | — | mailcow API key (`X-API-Key`). |
| `MAILFOLD_MAILCOW_INSECURE_TLS` | `false` | Skip TLS verification (dev only). |
| `MAILFOLD_FRONTEND_DIR` | `./frontend/dist` | Built SPA directory (optional). |

### Run

```bash
cp .env.example .env   # then edit values
make run               # or: cd backend && go run ./cmd/mailfold
```

### Current API

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/health` | Liveness probe. |
| `GET` | `/api/domains` | List mail domains. |
| `GET` | `/api/mailboxes` | List mailboxes. |

## Frontend

The UI is being built from a dedicated design project and will live in
[`frontend/`](frontend/). See [`frontend/README.md`](frontend/README.md).

## Target deployment

- Host: `mail.cortexus.ru` (mailcow on UpCloud, `/opt/mailcow-dockerized`).
- Serviced mail domains: `cortexus.ru`, `agentum.digital`, `proteus-vpn.cloud`.

## License

MIT — see [LICENSE](LICENSE).
