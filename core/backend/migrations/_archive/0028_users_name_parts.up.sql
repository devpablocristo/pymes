ALTER TABLE users
    ADD COLUMN IF NOT EXISTS given_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS family_name text NOT NULL DEFAULT '';

-- Rellenar desde name existente (primera palabra / resto).
UPDATE users
SET
    given_name = COALESCE(NULLIF(split_part(btrim(name), ' ', 1), ''), ''),
    family_name = CASE
        WHEN position(' ' IN btrim(name)) = 0 THEN ''
        ELSE btrim(substr(btrim(name), length(split_part(btrim(name), ' ', 1)) + 2))
    END;
