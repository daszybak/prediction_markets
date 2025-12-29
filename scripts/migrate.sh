#!/bin/sh
set -eu

DB_HOST="${POSTGRES_HOST:-timescaledb}"
DB_USER="${POSTGRES_USER:-prediction}"
DB_NAME="${POSTGRES_DB:-prediction}"
DB_PASS="${POSTGRES_PASSWORD:-}"

DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:5432/${DB_NAME}?sslmode=disable"

echo "Waiting for database..."
until pg_isready -h "$DB_HOST" -U "$DB_USER" -q; do
    sleep 1
done

echo "Running migrations..."
migrate -path /app/db/migrations -database "$DATABASE_URL" up
echo "Migrations complete."
