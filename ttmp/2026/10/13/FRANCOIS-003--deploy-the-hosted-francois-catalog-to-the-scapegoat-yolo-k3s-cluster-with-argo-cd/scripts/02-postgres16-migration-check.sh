#!/usr/bin/env bash
# The yolo cluster runs postgres:16-alpine; local development used 17.
# Applies the catalog migrations to a throwaway PostgreSQL 16 container and
# runs the database-backed Go tests against it.
set -euo pipefail
name=francois-pg16-check
docker rm -f "$name" >/dev/null 2>&1 || true
docker run -d --name "$name" -e POSTGRES_USER=francois -e POSTGRES_PASSWORD=francois \
  -e POSTGRES_DB=francois -p 127.0.0.1:5436:5432 postgres:16-alpine >/dev/null
trap 'docker rm -f "$name" >/dev/null' EXIT
for _ in $(seq 1 60); do docker exec "$name" pg_isready -U francois -d francois >/dev/null 2>&1 && break; sleep 1; done
sleep 2
docker exec "$name" psql -U francois -d francois -tAc 'select version()'
go run ./cmd/francois catalog migrate --database-url "postgres://francois:francois@127.0.0.1:5436/francois?sslmode=disable"
TEST_DATABASE_URL="postgres://francois:francois@127.0.0.1:5436/francois?sslmode=disable" \
  go test -count=1 ./pkg/catalog/...
