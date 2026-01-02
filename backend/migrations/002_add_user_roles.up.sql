-- Add role column to users table
ALTER TABLE users
ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user';

-- Add check constraint to ensure valid roles
ALTER TABLE users
ADD CONSTRAINT valid_role CHECK (role IN ('user', 'admin'));

-- Create index for role-based queries
CREATE INDEX idx_users_role ON users(role);

-- Promote franssjos@gmail.com to admin
UPDATE users
SET role = 'admin'
WHERE email = 'franssjos@gmail.com'
AND deleted_at IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN users.role IS 'User role: user (default) or admin';
