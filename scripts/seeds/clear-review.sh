#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

run_review_sql_inline "
DELETE FROM delegations
WHERE owner_id = 'pymes-platform'
  AND owner_type = 'service'
  AND agent_id = 'pymes-ai'
  AND agent_type = 'service';

DELETE FROM policies
WHERE name IN (
  'auto-allow-appointment-book',
  'auto-allow-appointment-reschedule',
  'require-approval-appointment-cancel',
  'auto-allow-small-discount',
  'require-approval-large-discount',
  'deny-refund',
  'auto-allow-payment-link',
  'auto-allow-notification',
  'require-approval-bulk-notification',
  'auto-allow-sale',
  'auto-allow-quote'
);

DELETE FROM action_types
WHERE name IN (
  'appointment.book',
  'appointment.reschedule',
  'appointment.cancel',
  'discount.apply',
  'payment_link.generate',
  'refund.create',
  'notification.send',
  'notification.bulk_send',
  'sale.create',
  'quote.create',
  'cashflow.movement',
  'work_order.delay_notify',
  'vehicle.service_reminder',
  'purchase.draft',
  'procurement.request',
  'procurement.submit'
);
"
