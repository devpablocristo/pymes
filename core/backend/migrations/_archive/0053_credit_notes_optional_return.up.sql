-- Notas de crédito sin devolución (alta manual / importación).
ALTER TABLE credit_notes ALTER COLUMN return_id DROP NOT NULL;
