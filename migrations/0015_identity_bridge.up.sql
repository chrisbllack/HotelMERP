-- 0015 Identity bridge: per-property role assignments
-- The auth layer (Login) loads roles + permission sets from this table on
-- both storage backends; SQLite carries the same table in its embedded schema.

CREATE TABLE IF NOT EXISTS user_property_roles (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    property_id VARCHAR(255),
    role VARCHAR(100) NOT NULL,
    permissions TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upr_user_property ON user_property_roles (user_id, property_id);

-- Grant the seeded administrator the wildcard permission set for the first
-- property. Harmless when the seeded user does not exist.
INSERT INTO user_property_roles (id, user_id, property_id, role, permissions)
SELECT 'role-admin-pg-01', u.id::text, p.id::text, 'admin', '["*"]'
FROM users u
CROSS JOIN properties p
WHERE u.email = 'admin@infineon.com'
LIMIT 1
ON CONFLICT DO NOTHING;
