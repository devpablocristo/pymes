#!/usr/bin/env bash

initial_seed_audit_end_at() {
  local completed_at="$1" grace_seconds="$2" completed_epoch
  [[ "$completed_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ &&
     "$grace_seconds" =~ ^[1-9][0-9]*$ ]] || return 1
  completed_epoch=$(date -u -d "$completed_at" +%s) || return 1
  date -u -d "@$((completed_epoch + grace_seconds))" +%Y-%m-%dT%H:%M:%SZ
}
