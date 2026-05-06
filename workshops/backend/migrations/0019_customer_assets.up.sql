CREATE TABLE IF NOT EXISTS workshops.customer_assets (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    asset_type text NOT NULL,
    customer_id uuid,
    customer_name text NOT NULL DEFAULT '',
    label text NOT NULL DEFAULT '',
    brand text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    serial_number text NOT NULL DEFAULT '',
    year integer NOT NULL DEFAULT 0,
    color text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customer_assets_org_type_active
    ON workshops.customer_assets (org_id, asset_type, archived_at);

CREATE INDEX IF NOT EXISTS idx_customer_assets_org_type_id
    ON workshops.customer_assets (org_id, asset_type, id DESC);

INSERT INTO workshops.customer_assets (
    id, org_id, asset_type, customer_id, customer_name, label, brand, model, serial_number,
    year, color, notes, metadata, is_favorite, tags, archived_at, created_at, updated_at
)
SELECT
    id,
    org_id,
    'vehicle',
    customer_id,
    customer_name,
    COALESCE(NULLIF(TRIM(license_plate), ''), NULLIF(TRIM(CONCAT_WS(' ', make, model)), ''), id::text),
    make,
    model,
    vin,
    year,
    color,
    notes,
    jsonb_build_object(
        'license_plate', license_plate,
        'vin', vin,
        'kilometers', kilometers
    ),
    is_favorite,
    tags,
    archived_at,
    created_at,
    updated_at
FROM workshops.vehicles
ON CONFLICT (id) DO UPDATE
    SET org_id = EXCLUDED.org_id,
        asset_type = EXCLUDED.asset_type,
        customer_id = EXCLUDED.customer_id,
        customer_name = EXCLUDED.customer_name,
        label = EXCLUDED.label,
        brand = EXCLUDED.brand,
        model = EXCLUDED.model,
        serial_number = EXCLUDED.serial_number,
        year = EXCLUDED.year,
        color = EXCLUDED.color,
        notes = EXCLUDED.notes,
        metadata = EXCLUDED.metadata,
        is_favorite = EXCLUDED.is_favorite,
        tags = EXCLUDED.tags,
        archived_at = EXCLUDED.archived_at,
        updated_at = EXCLUDED.updated_at;

INSERT INTO workshops.customer_assets (
    id, org_id, asset_type, customer_id, customer_name, label, brand, model, serial_number,
    year, color, notes, metadata, is_favorite, tags, archived_at, created_at, updated_at
)
SELECT
    id,
    org_id,
    'bicycle',
    customer_id,
    customer_name,
    COALESCE(NULLIF(TRIM(CONCAT_WS(' ', brand, model)), ''), NULLIF(TRIM(frame_number), ''), id::text),
    brand,
    model,
    frame_number,
    0,
    color,
    notes,
    jsonb_build_object(
        'frame_number', frame_number,
        'bike_type', bike_type,
        'size', size,
        'wheel_size_inches', wheel_size_inches,
        'ebike_notes', ebike_notes
    ),
    is_favorite,
    tags,
    archived_at,
    created_at,
    updated_at
FROM workshops.bicycles
ON CONFLICT (id) DO UPDATE
    SET org_id = EXCLUDED.org_id,
        asset_type = EXCLUDED.asset_type,
        customer_id = EXCLUDED.customer_id,
        customer_name = EXCLUDED.customer_name,
        label = EXCLUDED.label,
        brand = EXCLUDED.brand,
        model = EXCLUDED.model,
        serial_number = EXCLUDED.serial_number,
        year = EXCLUDED.year,
        color = EXCLUDED.color,
        notes = EXCLUDED.notes,
        metadata = EXCLUDED.metadata,
        is_favorite = EXCLUDED.is_favorite,
        tags = EXCLUDED.tags,
        archived_at = EXCLUDED.archived_at,
        updated_at = EXCLUDED.updated_at;
