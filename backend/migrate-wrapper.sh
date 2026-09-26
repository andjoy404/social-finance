#!/bin/sh
# Wrapper for golang-migrate that supplies default source/database flags.
# Environment:
#   DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE (from docker-compose or .env)
# This allows `docker compose run --rm migrate [command]` to work without
# repeating -source and -database flags for every invocation.
set -e

migrate \
  -source file:///migrations \
  -database "postgres://${DB_USER:-social_finance}:${DB_PASSWORD:-social_finance_dev_password}@postgres:5432/${DB_NAME:-social_finance}?sslmode=disable" \
  "$@"

# If running 'up' on default development database, also ensure social_finance_test is up to date
if [ "$1" = "up" ] && [ "${DB_NAME:-social_finance}" = "social_finance" ]; then
  migrate \
    -source file:///migrations \
    -database "postgres://${DB_USER:-social_finance}:${DB_PASSWORD:-social_finance_dev_password}@postgres:5432/social_finance_test?sslmode=disable" \
    up || true
fi
