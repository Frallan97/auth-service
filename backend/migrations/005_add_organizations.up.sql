-- Add super admin flag to users table
ALTER TABLE users ADD COLUMN is_super_admin BOOLEAN DEFAULT false NOT NULL;
CREATE INDEX idx_users_is_super_admin ON users(is_super_admin);

-- Promote existing admin users to super admins
UPDATE users SET is_super_admin = true WHERE role = 'admin';

COMMENT ON COLUMN users.is_super_admin IS 'Global administrator with access to all organizations and system management';

-- Organizations table
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by UUID,
    CONSTRAINT fk_org_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Indexes
CREATE INDEX idx_organizations_slug ON organizations(slug);
CREATE INDEX idx_organizations_is_active ON organizations(is_active);

-- Update trigger
CREATE TRIGGER update_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- User-Organizations junction table with org-scoped roles
CREATE TABLE user_organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_user_org_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_org_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT valid_org_role CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    CONSTRAINT unique_user_org UNIQUE (user_id, organization_id)
);

-- Indexes
CREATE INDEX idx_user_organizations_user_id ON user_organizations(user_id);
CREATE INDEX idx_user_organizations_organization_id ON user_organizations(organization_id);
CREATE INDEX idx_user_organizations_role ON user_organizations(role);

-- Update trigger
CREATE TRIGGER update_user_organizations_updated_at
    BEFORE UPDATE ON user_organizations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- User application logins tracking
CREATE TABLE user_application_logins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    application_id UUID NOT NULL,
    organization_id UUID,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_user_app_login_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_app_login_app FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_app_login_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE SET NULL
);

-- Indexes for efficient queries
CREATE INDEX idx_user_app_logins_user_id ON user_application_logins(user_id);
CREATE INDEX idx_user_app_logins_app_id ON user_application_logins(application_id);
CREATE INDEX idx_user_app_logins_org_id ON user_application_logins(organization_id);
CREATE INDEX idx_user_app_logins_created_at ON user_application_logins(created_at DESC);

-- Organization-Application access control (optional but useful)
CREATE TABLE organization_applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    application_id UUID NOT NULL,
    is_active BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_org_app_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_org_app_app FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    CONSTRAINT unique_org_app UNIQUE (organization_id, application_id)
);

-- Indexes
CREATE INDEX idx_org_apps_organization_id ON organization_applications(organization_id);
CREATE INDEX idx_org_apps_application_id ON organization_applications(application_id);

-- Comments for documentation
COMMENT ON TABLE organizations IS 'Business entities/teams that group users with role-based access control';
COMMENT ON TABLE user_organizations IS 'Junction table mapping users to organizations with organization-specific roles';
COMMENT ON COLUMN user_organizations.role IS 'Organization-scoped role: owner (full control), admin (manage members), member (standard access), viewer (read-only)';
COMMENT ON TABLE user_application_logins IS 'Audit trail of user logins to registered applications';
COMMENT ON TABLE organization_applications IS 'Controls which organizations have access to which applications';
