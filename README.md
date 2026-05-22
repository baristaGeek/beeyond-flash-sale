# Flash Sale Reservation System

High-concurrency stock reservation service. Go + PostgreSQL backend, React + Vite + TypeScript frontend.

Architecture, concurrency strategy, and API contract live under [`specs/001-flash-sale-reservation/`](./specs/001-flash-sale-reservation/) (`spec.md`, `plan.md`, `research.md`, `contracts/api.md`, `quickstart.md`).

## Prerequisites

- Go 1.22+
- Node.js 20+
- Docker Desktop

## Run

`run-backend` and `run-frontend` are long-running processes that block their terminal, so use two terminals.

**Terminal 1 — setup + backend**

```bash
make up              # start Postgres (detached, returns immediately)
make migrate-up      # apply schema
make seed            # insert demo sale (run once per fresh DB)
make run-backend     # long-running → http://localhost:8080
```

**Terminal 2 — frontend**

```bash
make run-frontend    # long-running → http://localhost:5173
```

Open <http://localhost:5173>.

Notes:

- If `make migrate-up` fails with a connection error right after `make up`, give Postgres a couple of seconds to accept connections and retry.
- To start over from a clean DB, run `make migrate-reset` then `make seed`.
- On subsequent runs (Postgres already up, schema applied, data seeded), only the two `run-*` commands are needed.
- Stop with `Ctrl+C` in each terminal, then `make down` to stop Postgres.

## Tests

```bash
make test              # backend unit tests
make test-race         # backend tests with -race
make test-integration  # real Postgres + HTTP (requires `make up`)
make loadtest          # 100 concurrent reservations against capacity=10
```

The load test exits non-zero if the no-oversell invariant is violated.

## Teardown

```bash
make down              # stop Postgres (add `-v` via `docker compose down -v` to wipe data)
```
