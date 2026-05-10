-- 0051: Rename role names to English.
UPDATE roles SET name = 'seller',      description = 'Sales and commercial management'  WHERE name = 'vendedor';
UPDATE roles SET name = 'cashier',     description = 'Payments and cash register'        WHERE name = 'cajero';
UPDATE roles SET name = 'accountant',  description = 'Reports and accounting'            WHERE name = 'contador';
UPDATE roles SET name = 'warehouse',   description = 'Inventory and products'            WHERE name = 'almacenero';
