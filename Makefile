# Loads .env into every recipe below (export makes each var available to
# the shell commands that follow, not just to `make` itself).
include .env
export

.PHONY: up down seed reseed logs psql

## Start Postgres (detached)
up:
	docker compose up -d

## Stop Postgres (keeps data volume)
down:
	docker compose down

## Run the seed script (DATABASE_URL comes from .env via `include` above)
seed:
	go run ./seed

## Wipe local data and reseed from scratch
reseed:
	docker compose down
	rm -rf ./pgdata
	docker compose up -d
	@echo "waiting for postgres to become healthy..."
	@until [ "$$(docker inspect -f '{{.State.Health.Status}}' export_service_db 2>/dev/null)" = "healthy" ]; do sleep 1; done
	go run ./seed

## Tail logs
logs:
	docker compose logs -f

## Open a psql shell into the running container
psql:
	docker exec -it export_service_db psql -U $(DB_USER) -d $(DB_NAME)