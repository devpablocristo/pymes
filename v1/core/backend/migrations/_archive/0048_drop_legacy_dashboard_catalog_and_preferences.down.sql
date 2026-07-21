CREATE TABLE IF NOT EXISTS dashboard_widgets_catalog (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    widget_key text NOT NULL UNIQUE,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    domain text NOT NULL,
    kind text NOT NULL,
    default_width integer NOT NULL,
    default_height integer NOT NULL,
    min_width integer NOT NULL DEFAULT 2,
    min_height integer NOT NULL DEFAULT 2,
    max_width integer NOT NULL DEFAULT 12,
    max_height integer NOT NULL DEFAULT 8,
    allowed_roles_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    required_scopes_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    supported_contexts_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    settings_schema_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    data_endpoint text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_dashboard_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    user_actor text NOT NULL UNIQUE,
    default_context text NOT NULL DEFAULT 'home',
    preferences_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
