ALTER TABLE customers
    DROP COLUMN IF EXISTS price_list_id;

ALTER TABLE quotes
    DROP COLUMN IF EXISTS discount_total,
    DROP COLUMN IF EXISTS discount_value,
    DROP COLUMN IF EXISTS discount_type;

ALTER TABLE sales
    DROP COLUMN IF EXISTS discount_total,
    DROP COLUMN IF EXISTS discount_value,
    DROP COLUMN IF EXISTS discount_type,
    DROP COLUMN IF EXISTS payment_status,
    DROP COLUMN IF EXISTS amount_paid;

ALTER TABLE quote_items
    DROP COLUMN IF EXISTS discount_value,
    DROP COLUMN IF EXISTS discount_type;

ALTER TABLE sale_items
    DROP COLUMN IF EXISTS discount_value,
    DROP COLUMN IF EXISTS discount_type;

DROP TABLE IF EXISTS appointment_slots;
DROP TABLE IF EXISTS appointments;
DROP TABLE IF EXISTS recurring_expenses;
DROP TABLE IF EXISTS price_list_items;
DROP TABLE IF EXISTS price_lists;
DROP TABLE IF EXISTS credit_notes;
DROP TABLE IF EXISTS return_items;
DROP TABLE IF EXISTS returns;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS account_movements;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS purchase_items;
DROP TABLE IF EXISTS purchases;
