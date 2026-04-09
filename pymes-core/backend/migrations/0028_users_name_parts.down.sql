-- Reverso de 0028_users_name_parts.up.sql
-- WARNING: borra given_name y family_name con todos sus valores.
-- El campo `name` original NO se modifica en el up, así que sigue intacto.

ALTER TABLE users DROP COLUMN IF EXISTS family_name;
ALTER TABLE users DROP COLUMN IF EXISTS given_name;
