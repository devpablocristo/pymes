-- Renombrar appointment_id → booking_id en work orders.
ALTER TABLE workshops.work_orders RENAME COLUMN appointment_id TO booking_id;
ALTER TABLE workshops.bike_work_orders RENAME COLUMN appointment_id TO booking_id;
