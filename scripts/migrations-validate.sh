#!/bin/sh
# migrations-validate.sh — valida que las migraciones de pymes-core levantan
# desde DB vacía y producen el schema esperado (vs baseline _reference_schema.sql).
#
# Pasos:
#   1. Levanta un postgres efímero en un contenedor temporal (puerto 55432).
#   2. Conecta el backend cp-backend (en modo compile only) para correr migrations.Run().
#   3. Hace pg_dump --schema-only del resultado.
#   4. Compara contra pymes-core/backend/migrations/_squashed/_reference_schema.sql.
#   5. Limpia el contenedor temporal.
#
# Falla con exit 1 si hay drift.
#
# Uso:
#   scripts/migrations-validate.sh
#
# Pre-req: docker + go disponibles en PATH.

set -e

REF_SCHEMA="pymes-core/backend/migrations/_squashed/_reference_schema.sql"
TMP_DUMP="/tmp/migrations-validate-$$.sql"
TMP_CONTAINER="pymes-migrations-validate-$$"
DB_NAME="pymes_validate"
DB_USER="postgres"
DB_PASS="postgres"
HOST_PORT="55432"

cleanup() {
    docker rm -f "$TMP_CONTAINER" >/dev/null 2>&1 || true
    rm -f "$TMP_DUMP"
}
trap cleanup EXIT

if [ ! -f "$REF_SCHEMA" ]; then
    echo "ERROR: baseline ausente en $REF_SCHEMA"
    echo "Ejecutá scripts/migrations-snapshot.sh primero (con un stack actual sano)."
    exit 1
fi

echo "1) Levantando postgres efímero en :$HOST_PORT..."
docker run -d --rm \
    --name "$TMP_CONTAINER" \
    -e "POSTGRES_USER=$DB_USER" \
    -e "POSTGRES_PASSWORD=$DB_PASS" \
    -e "POSTGRES_DB=$DB_NAME" \
    -p "$HOST_PORT:5432" \
    postgres:16-alpine >/dev/null

# Espera readiness
echo "   esperando readiness..."
for i in $(seq 1 30); do
    if docker exec "$TMP_CONTAINER" pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

echo "2) Corriendo migraciones de pymes-core (vía go test util)..."
DATABASE_URL="postgres://$DB_USER:$DB_PASS@localhost:$HOST_PORT/$DB_NAME?sslmode=disable" \
    go test -run '^TestMigrationsBootstrap$' ./pymes-core/backend/migrations/... \
    -count=1 -timeout 5m

echo "3) Snapshot del resultado..."
docker exec "$TMP_CONTAINER" pg_dump \
    -U "$DB_USER" \
    -d "$DB_NAME" \
    --schema-only \
    --no-owner \
    --no-privileges \
    --no-comments \
    --exclude-schema=information_schema \
    --exclude-schema='pg_*' \
    > "$TMP_DUMP"

echo "4) Diff vs baseline..."
if diff -u "$REF_SCHEMA" "$TMP_DUMP" >/dev/null 2>&1; then
    echo "OK: schema bootstrap == baseline (sin drift)."
    exit 0
fi

echo "DRIFT DETECTADO:"
diff -u "$REF_SCHEMA" "$TMP_DUMP" | head -60
echo "..."
echo ""
echo "Para regenerar la baseline (si el cambio es intencional):"
echo "  scripts/migrations-snapshot.sh"
exit 1
