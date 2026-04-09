DROP INDEX IF EXISTS idx_scheduling_waitlist_entries_pending;
DROP INDEX IF EXISTS idx_scheduling_waitlist_entries_scope;
DROP INDEX IF EXISTS uidx_scheduling_waitlist_entries_idempotency;
DROP TABLE IF EXISTS scheduling_waitlist_entries;

DROP INDEX IF EXISTS idx_scheduling_booking_action_tokens_active;
DROP INDEX IF EXISTS idx_scheduling_booking_action_tokens_booking;
DROP INDEX IF EXISTS uidx_scheduling_booking_action_tokens_hash;
DROP TABLE IF EXISTS scheduling_booking_action_tokens;

ALTER TABLE scheduling_queue_tickets
    DROP COLUMN IF EXISTS customer_email;

ALTER TABLE scheduling_bookings
    DROP COLUMN IF EXISTS reminder_sent_at,
    DROP COLUMN IF EXISTS customer_email;

ALTER TABLE scheduling_services
    DROP COLUMN IF EXISTS allow_waitlist,
    DROP COLUMN IF EXISTS min_cancel_notice_minutes;
