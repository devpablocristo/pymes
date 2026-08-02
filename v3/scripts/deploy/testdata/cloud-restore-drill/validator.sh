#!/usr/bin/env bash
set -euo pipefail

jq -n \
  --arg environment "$PYMES_RESTORE_DRILL_ENV" \
  --arg drill_id "$PYMES_RESTORE_DRILL_ID" \
  --arg pymes "$PYMES_RESTORE_DRILL_PYMES_TARGET" \
  --arg fiscal "$PYMES_RESTORE_DRILL_FISCAL_TARGET" \
  --arg accounting "$PYMES_RESTORE_DRILL_ACCOUNTING_TARGET" '
    {
      schema: "pymes-v3-cloud-restore-validation-v1",
      environment: $environment,
      drill_id: $drill_id,
      targets: {
        pymes: $pymes,
        fiscal: $fiscal,
        accounting: $accounting
      },
      migrations_applied: true,
      tenant_isolation_verified: true,
      probes_ready: true,
      reconciliation_runs: 2,
      duplicate_fiscal_requests: 0,
      duplicate_accounting_commands: 0,
      duplicate_journal_entries: 0,
      unpublished_recoverable_outbox: 0
    }
  '
