# Seed script

Populates the database with realistic, deliberately skewed fake data:

| Table         | Rows       |
|---------------|------------|
| sellers       | 500        |
| customers     | 200,000    |
| products      | 2,000      |
| orders        | 1,000,000  |
| order_items   | 3,000,000  |

**Skew:** the top 5% of sellers (25 sellers) receive 60% of all
`order_items`. This is deliberate - it's what makes an unindexed query for a
popular seller genuinely slow

Dates (`order_purchase_timestamp`) are spread randomly over the last 2 years,
so a date-range filter has real, unevenly distributed data to work with.

## Prerequisites

- Migrations already applied (`docker compose up` with a fresh `./pgdata`
  runs `001_create_tables.sql` and `002_create_jobs.sql` automatically).
- Dependencies:
  ```bash
  go get github.com/jackc/pgx/v5
  go get github.com/brianvoe/gofakeit/v7
  ```

## Running it

```bash
export DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/export_service"
go run ./seed
```

Expect this to take a few minutes - `order_items` alone is 3 million rows,
inserted in batches of 50,000 via `COPY`. Progress prints as it goes.

## Re-running

This script does **not** truncate existing tables first. To reseed from
scratch:

```bash
docker compose down
rm -rf ./pgdata
docker compose up -d
# wait for healthcheck to pass, then:
go run ./seed
```

## Verifying afterward

```sql
SELECT count(*) FROM order_items;                 -- ~3,000,000
SELECT seller_id, count(*) FROM order_items
  GROUP BY seller_id ORDER BY count(*) DESC LIMIT 5; -- confirm the skew
```