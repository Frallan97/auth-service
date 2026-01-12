-- Drop the allowed_origins table
DROP TRIGGER IF EXISTS update_allowed_origins_updated_at ON allowed_origins;
DROP INDEX IF EXISTS idx_allowed_origins_is_active;
DROP INDEX IF EXISTS idx_allowed_origins_origin;
DROP TABLE IF EXISTS allowed_origins;
