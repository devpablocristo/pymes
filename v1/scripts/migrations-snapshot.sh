#!/bin/sh
# migrations-snapshot.sh — exporta el schema final de core (post-migraciones)
# como baseline de referencia para drift checks.
#
# Uso:
#   scripts/migrations-snapshot.sh                          # snapshot del estado actual del repo
#   scripts/migrations-snapshot.sh path/to/output.sql       # output personalizado
#
# Pre-req: docker compose running con `postgres` + `cp-backend` healthy
# (`make up`). El backend debe haber aplicado todas las migraciones.

set -e

OUTPUT="${1:-core/backend/migrations/_squashed/_reference_schema.sql}"
SERVICE="postgres"
DB="${PGDATABASE:-pymes}"
USER="${PGUSER:-postgres}"

mkdir -p "$(dirname "$OUTPUT")"

echo "Snapshot del schema de '$DB' a '$OUTPUT'..."
docker compose exec -T "$SERVICE" pg_dump \
    -U "$USER" \
    -d "$DB" \
    --schema-only \
    --no-owner \
    --no-privileges \
    --no-comments \
    --exclude-schema=information_schema \
    --exclude-schema='pg_*' \
    > "$OUTPUT"

echo "OK: $(wc -l < "$OUTPUT") líneas escritas en $OUTPUT"
