CREATE TABLE IF NOT EXISTS dashboard_default_layouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    layout_key text NOT NULL UNIQUE,
    context text NOT NULL,
    name text NOT NULL,
    items_json jsonb NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_default_layouts_context_active
    ON dashboard_default_layouts(context)
    WHERE is_active = true;

CREATE TABLE IF NOT EXISTS user_dashboard_layouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    user_actor text NOT NULL,
    context text NOT NULL,
    layout_version integer NOT NULL DEFAULT 1,
    items_json jsonb NOT NULL,
    last_applied_default_layout_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_actor, context)
);

CREATE INDEX IF NOT EXISTS idx_user_dashboard_layouts_user_id ON user_dashboard_layouts(user_id);
