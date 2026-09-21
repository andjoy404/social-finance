#!/bin/sh
# Wrapper for golang-migrate that supplies default source/database flags.
# Environment:
#   DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE (from docker-compose or .env)
# This allows `docker compose run --rm migrate [command]` to work without
# repeating -source and -database flags for every invocation.
exec migrate \
  -source file:///migrations \
  -database "postgres://${DB_USER:-social_finance}:${DB_PASSWORD:-social_finance_dev_password}@postgres:5432/${DB_NAME:-social_finance}?sslmode=disable" \
  "$@"
