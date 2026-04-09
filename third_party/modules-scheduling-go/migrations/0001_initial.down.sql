ALTER TABLE IF EXISTS scheduling_bookings DROP CONSTRAINT IF EXISTS scheduling_bookings_no_overlap;

DROP TABLE IF EXISTS scheduling_queue_tickets;
DROP TABLE IF EXISTS scheduling_queues;
DROP TABLE IF EXISTS scheduling_bookings;
DROP TABLE IF EXISTS scheduling_blocked_ranges;
DROP TABLE IF EXISTS scheduling_availability_rules;
DROP TABLE IF EXISTS scheduling_service_resources;
DROP TABLE IF EXISTS scheduling_resources;
DROP TABLE IF EXISTS scheduling_services;
DROP TABLE IF EXISTS scheduling_branches;
