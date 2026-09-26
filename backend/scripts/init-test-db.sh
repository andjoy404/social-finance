#!/bin/sh
set -e

# Create test database if it does not already exist
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    SELECT 'CREATE DATABASE social_finance_test'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'social_finance_test')\gexec
    GRANT ALL PRIVILEGES ON DATABASE social_finance_test TO "$POSTGRES_USER";
EOSQL
