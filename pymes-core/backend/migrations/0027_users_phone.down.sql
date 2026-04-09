-- Reverso de 0027_users_phone.up.sql
-- WARNING: borra la columna phone con todos sus valores.

ALTER TABLE users DROP COLUMN IF EXISTS phone;
