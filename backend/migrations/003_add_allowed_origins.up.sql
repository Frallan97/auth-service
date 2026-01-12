-- Allowed origins table for CORS management
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

-- Index for better query performance
CREATE INDEX idx_allowed_origins_is_active ON allowed_origins(is_active);
CREATE INDEX idx_allowed_origins_origin ON allowed_origins(origin);

-- Update trigger for updated_at
CREATE TRIGGER update_allowed_origins_updated_at
    BEFORE UPDATE ON allowed_origins
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default origins from common development environments
INSERT INTO allowed_origins (origin, description, is_active) VALUES
    ('http://localhost:3000', 'Local development (React default)', true),
    ('http://localhost:5173', 'Local development (Vite default)', true)
ON CONFLICT (origin) DO NOTHING;
