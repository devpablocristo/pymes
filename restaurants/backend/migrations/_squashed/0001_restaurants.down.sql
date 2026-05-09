-- 0001_restaurants.down.sql

DROP TRIGGER IF EXISTS trg_reservations_updated_at ON restaurant.reservations;
DROP TRIGGER IF EXISTS trg_table_sessions_updated_at ON restaurant.table_sessions;
DROP TRIGGER IF EXISTS trg_dining_tables_updated_at ON restaurant.dining_tables;
DROP TRIGGER IF EXISTS trg_dining_areas_updated_at ON restaurant.dining_areas;

DROP TABLE IF EXISTS restaurant.reservations;
DROP TABLE IF EXISTS restaurant.table_sessions;
DROP TABLE IF EXISTS restaurant.dining_tables;
DROP TABLE IF EXISTS restaurant.dining_areas;

DROP SCHEMA IF EXISTS restaurant;
