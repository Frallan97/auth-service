-- Recreate allowed_origins table from applications
CREATE TABLE allowed_origins (
    id SERIAL PRIMARY KEY,
    origin VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by UUID,
    CONSTRAINT fk_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Recreate indexes
CREATE INDEX idx_allowed_origins_is_active ON allowed_origins(is_active);
CREATE INDEX idx_allowed_origins_origin ON allowed_origins(origin);

-- Recreate update trigger
CREATE TRIGGER update_allowed_origins_updated_at
    BEFORE UPDATE ON allowed_origins
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Migrate data back from applications to allowed_origins
INSERT INTO allowed_origins (origin, description, is_active, created_at, updated_at, created_by)
SELECT
    origin,
    description,
    is_active,
    created_at,
    updated_at,
    created_by
FROM applications;

-- Drop applications table
DROP TABLE applications;
