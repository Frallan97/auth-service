-- Drop tables in reverse order (respecting foreign key constraints)
DROP TABLE IF EXISTS organization_applications;
DROP TABLE IF EXISTS user_application_logins;
DROP TABLE IF EXISTS user_organizations;
DROP TABLE IF EXISTS organizations;

-- Remove super admin column from users
DROP INDEX IF EXISTS idx_users_is_super_admin;
ALTER TABLE users DROP COLUMN IF EXISTS is_super_admin;
