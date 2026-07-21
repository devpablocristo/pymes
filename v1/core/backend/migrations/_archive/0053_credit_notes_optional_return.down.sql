-- Elimina notas manuales sin devolución antes de volver a NOT NULL.
DELETE FROM credit_notes WHERE return_id IS NULL;
ALTER TABLE credit_notes ALTER COLUMN return_id SET NOT NULL;
