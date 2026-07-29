# Async Bulk Export Service

A Go backend service for exporting large relational datasets (CSV) asynchronously - built around a bounded worker pool, backpressure, and graceful shutdown, with a PostgreSQL-backed job queue.

## The problem

Generating a large export (e.g., a seller's full order history) synchronously inside an HTTP request doesn't scale - long-running queries risk timeouts and block the server from handling other requests. This service decouples "accept the export request" from "do the work," processing exports asynchronously via a background worker pool while the client polls for status.

## Architecture

```
Client
  │  POST /exports {seller_id, date_from, date_to}
  ▼
HTTP API ── writes job (status: pending) ──► PostgreSQL
  │
  │  pushes job onto bounded channel
  ▼
Worker Pool (fixed-size goroutine pool)
  │
  ├─ picks up job
  ├─ runs multi-table export query
  ├─ streams results to CSV
  └─ updates job status in PostgreSQL

Client
  │  GET /exports/:id           → poll status
  │  GET /exports/:id/download  → fetch completed file
```

## Tech stack

- **Go** - HTTP API, worker pool, concurrency handling
- **PostgreSQL** - relational data store and durable job-state tracking
- **Docker Compose** - local Postgres environment

## Key design decisions

- **Bounded worker pool**: a fixed number of goroutines process jobs concurrently, rather than spawning a goroutine per request - this keeps resource usage (DB connections, memory) predictable under load.
- **Backpressure**: the job queue has a fixed capacity. Once full, new export requests are rejected with a clear "busy, try again" response instead of queuing indefinitely.
- **Graceful shutdown**: on shutdown signal, the service stops accepting new jobs but lets in-flight exports finish cleanly before exiting, avoiding corrupted output files.
- **Durable job state**: job status lives in PostgreSQL rather than only in memory, so it survives restarts and supports querying job history.

## Data

The database is seeded with realistic, relationally-consistent synthetic data (sellers, customers, products, orders, and order items) at a scale of several million rows, generated with [`gofakeit`](https://github.com/brianvoe/gofakeit). Seller order volume is deliberately distributed unevenly, reflecting how real marketplace order volume tends to concentrate among a subset of sellers - this keeps query behavior realistic at scale.

## Status

🚧 Work in progress. Current state: database schema, Docker-based local Postgres setup, and seed data generation are complete. HTTP API and worker pool implementation are in progress.

## Local setup

```bash
# 1. Copy environment config and fill in values
cp .env.example .env

# 2. Start PostgreSQL (applies migrations automatically on first run)
make up

# 3. Seed the database with sample data
make seed
```

## Available commands

| Command       | Description                                                 |
|---------------|---------------------------------------------------------------|
| `make up`     | Start PostgreSQL (detached)                                  |
| `make down`   | Stop PostgreSQL (keeps data)                                  |
| `make seed`   | Run the seed script against the running database              |
| `make reseed` | Wipe local data, restart Postgres, and reseed from scratch    |
| `make logs`   | Tail logs from all running services                           |
| `make psql`   | Open a `psql` shell into the running Postgres container       |

## Project structure

```
export-service/
├── cmd/server/       # application entrypoint
├── internal/
│   ├── api/           # HTTP handlers and routing
│   ├── worker/        # worker pool and export processing
│   ├── db/             # database connection and queries
│   └── config/          # environment/config loading
├── migrations/          # numbered SQL schema migrations
├── seed/                # one-off data seeding script
└── docker-compose.yml
```