-- 0010_dashboard.down.sql

DROP TRIGGER IF EXISTS trg_user_dashboard_layouts_updated_at ON user_dashboard_layouts;
DROP TRIGGER IF EXISTS trg_dashboard_widgets_catalog_updated_at ON dashboard_widgets_catalog;

DROP TABLE IF EXISTS user_dashboard_layouts;
DROP TABLE IF EXISTS dashboard_widgets_catalog;
