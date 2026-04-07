#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

TARGET_ORG_UUID="$(resolve_target_org_uuid)"

run_pymes_sql_inline "
DO \$\$
DECLARE
    v_org uuid := '${TARGET_ORG_UUID}';
    veh1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/vehicle/1');
    srv1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/service/oil');
    srv2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/service/brake');
    wo1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/1');
    wo2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/2');
BEGIN
    DELETE FROM workshops.work_order_items WHERE work_order_id IN (wo1, wo2);
    DELETE FROM workshops.work_orders WHERE id IN (wo1, wo2);
    DELETE FROM workshops.vehicles WHERE id = veh1;
    DELETE FROM services WHERE org_id = v_org AND id IN (srv1, srv2);
END \$\$;
"
