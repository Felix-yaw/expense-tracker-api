# expense-tracker-api

A REST API backend for the **Ledger · Expense Tracker** Flutter app, written in
Go with the standard library, JWT auth and SQLite.

## Stack

- **Go 1.27**, standard library `net/http` (method + path routing, Go 1.22+)
- **SQLite** via `modernc.org/sqlite` (pure Go — no CGO, no system deps)
- **JWT** access tokens (`github.com/golang-jwt/jwt/v5`) + **bcrypt** passwords
- Layered: `cmd` → `internal/api` → `internal/store`, with `auth`, `config`,
  `httpx` helpers

## Run

```bash
go run ./cmd/api           # http://localhost:8080
```

Configuration is via environment variables (see `.env.example`):

| Var | Default | Notes |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `DB_PATH` | `expense_tracker.db` | SQLite file |
| `JWT_SECRET` | dev default | **set a strong value in production** |
| `JWT_TTL` | `24h` | Token lifetime |
| `CORS_ORIGINS` | `*` | Comma-separated origins |

```bash
make test     # go test ./...
make vet      # go vet ./...
make docker   # build the image
docker compose up   # or run via compose
```

## API

Base path: `/api/v1`. All expense and budget routes require
`Authorization: Bearer <token>`.

### Auth

| Method | Path | Body | Description |
| --- | --- | --- | --- |
| `POST` | `/auth/register` | `{email, name, password}` | Create an account, returns `{token, user}` |
| `POST` | `/auth/login` | `{email, password}` | Returns `{token, user}` |
| `GET` | `/me` | — | Current user |

### Expenses

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/expenses` | List. Filters: `category`, `q`, `from`, `to`, `limit`, `offset` |
| `POST` | `/expenses` | Create `{title, amount, category, date, note}` |
| `GET` | `/expenses/{id}` | Fetch one |
| `PATCH` | `/expenses/{id}` | Partial update (any of the fields) |
| `DELETE` | `/expenses/{id}` | Delete (204) |
| `GET` | `/expenses/summary` | Totals overall + by category (`from`/`to` optional) |

### Budget & categories

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/budget` | `{monthlyLimit, categoryLimits}` |
| `PUT` | `/budget` | Upsert the budget |
| `GET` | `/categories` | The fixed category list |
| `GET` | `/healthz` | Liveness |

Categories: `food, transport, shopping, bills, health, entertainment, education, other`.

Dates accept either RFC3339 (`2026-01-15T10:30:00Z`) or `YYYY-MM-DD`.

## Example

```bash
BASE=http://localhost:8080/api/v1

TOKEN=$(curl -s -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","name":"Me","password":"password123"}' \
  | jq -r .token)

curl -s -X POST $BASE/expenses -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Coffee","amount":4.5,"category":"food","date":"2026-01-15"}'

curl -s $BASE/expenses/summary -H "Authorization: Bearer $TOKEN"
```

## Layout

```
cmd/api/            # main: config, store, server, graceful shutdown
internal/
  api/              # routes, middleware, handlers
  auth/             # JWT + bcrypt
  config/           # env loading
  httpx/            # JSON + middleware helpers
  models/           # domain types
  store/            # SQLite persistence + migrations
```
