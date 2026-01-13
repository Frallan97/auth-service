-- Create applications table (replaces allowed_origins)
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    origin VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    redirect_uris TEXT[],  -- Array of allowed redirect URIs
    is_active BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by UUID,
    CONSTRAINT fk_app_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Indexes for better query performance
CREATE INDEX idx_applications_slug ON applications(slug);
CREATE INDEX idx_applications_origin ON applications(origin);
CREATE INDEX idx_applications_is_active ON applications(is_active);

-- Update trigger for updated_at
CREATE TRIGGER update_applications_updated_at
    BEFORE UPDATE ON applications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Migrate data from allowed_origins to applications
INSERT INTO applications (id, name, slug, origin, description, is_active, created_at, updated_at, created_by)
SELECT
    gen_random_uuid() as id,
    origin as name,  -- Use origin URL as name initially (admins can rename)
    -- Generate slug from origin: remove protocol, replace dots/colons with dashes
    LOWER(REGEXP_REPLACE(REGEXP_REPLACE(origin, '^https?://', ''), '[.:]+', '-', 'g')) as slug,
    origin,
    description,
    is_active,
    created_at,
    updated_at,
    created_by
FROM allowed_origins;

-- Drop old table
DROP TABLE allowed_origins;

-- Add comment to clarify purpose
COMMENT ON TABLE applications IS 'Registered applications that use this auth service. Each application is automatically whitelisted for CORS.';
COMMENT ON COLUMN applications.origin IS 'CORS origin URL (e.g., https://example.com). Automatically whitelisted when application is active.';
COMMENT ON COLUMN applications.redirect_uris IS 'Array of allowed OAuth redirect URIs for this application.';
