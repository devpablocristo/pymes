#!/bin/sh
# Espera a que Postgres acepte conexiones antes de levantar air (evita FATAL "database system is starting up").
set -e
echo "waiting for postgres..."
until pg_isready -h "${POSTGRES_HOST:-postgres}" -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-pymes}"; do
  sleep 1
done
exec air -c .air.toml
