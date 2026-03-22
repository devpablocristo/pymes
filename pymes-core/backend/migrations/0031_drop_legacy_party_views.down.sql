CREATE VIEW customers AS
SELECT
    p.id,
    p.org_id,
    CASE WHEN p.party_type = 'organization' THEN 'company' ELSE 'person' END AS type,
    p.display_name AS name,
    p.tax_id,
    p.email,
    p.phone,
    p.address,
    p.notes,
    p.tags,
    p.metadata,
    p.created_at,
    p.updated_at,
    p.deleted_at,
    pr.price_list_id
FROM parties p
JOIN party_roles r
    ON r.party_id = p.id
   AND r.org_id = p.org_id
   AND r.role = 'customer'
   AND r.is_active = true
LEFT JOIN party_roles pr
    ON pr.party_id = p.id
   AND pr.org_id = p.org_id
   AND pr.role = 'customer';

CREATE VIEW suppliers AS
SELECT
    p.id,
    p.org_id,
    p.display_name AS name,
    p.tax_id,
    p.email,
    p.phone,
    p.address,
    COALESCE(r.metadata->>'contact_name', p.metadata->>'contact_name', '') AS contact_name,
    p.notes,
    p.tags,
    p.metadata,
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM parties p
JOIN party_roles r
    ON r.party_id = p.id
   AND r.org_id = p.org_id
   AND r.role = 'supplier'
   AND r.is_active = true;
