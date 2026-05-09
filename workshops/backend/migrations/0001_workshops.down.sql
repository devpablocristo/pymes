-- 0001_workshops.down.sql

DROP TRIGGER IF EXISTS trg_customer_assets_updated_at ON workshops.customer_assets;
DROP TRIGGER IF EXISTS trg_work_order_items_updated_at ON workshops.work_order_items;
DROP TRIGGER IF EXISTS trg_work_orders_updated_at ON workshops.work_orders;
DROP TRIGGER IF EXISTS trg_bicycles_updated_at ON workshops.bicycles;
DROP TRIGGER IF EXISTS trg_workshops_services_updated_at ON workshops.services;
DROP TRIGGER IF EXISTS trg_vehicles_updated_at ON workshops.vehicles;

DROP TABLE IF EXISTS workshops.work_order_assets;
DROP TABLE IF EXISTS workshops.customer_assets;
DROP TABLE IF EXISTS workshops.work_order_items;
DROP TABLE IF EXISTS workshops.work_orders;
DROP TABLE IF EXISTS workshops.bicycles;
DROP TABLE IF EXISTS workshops.services;
DROP TABLE IF EXISTS workshops.vehicles;

DROP SCHEMA IF EXISTS workshops;
