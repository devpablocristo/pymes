-- Estados canónicos y notificación WhatsApp (idempotencia).
ALTER TABLE workshops.work_orders
    ADD COLUMN IF NOT EXISTS ready_pickup_notified_at TIMESTAMPTZ NULL;

UPDATE workshops.work_orders SET status = 'diagnosing' WHERE status = 'diagnosis';
UPDATE workshops.work_orders SET status = 'ready_for_pickup' WHERE status = 'ready';
