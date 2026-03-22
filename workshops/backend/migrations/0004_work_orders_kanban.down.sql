UPDATE workshops.work_orders SET status = 'diagnosis' WHERE status = 'diagnosing';
UPDATE workshops.work_orders SET status = 'ready' WHERE status = 'ready_for_pickup';

ALTER TABLE workshops.work_orders
    DROP COLUMN IF EXISTS ready_pickup_notified_at;
