-- 1. Create new tables
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE oauth_provider_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    redirect_url TEXT NOT NULL,
    is_enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_app_provider ON oauth_provider_configs(app_id, provider);

-- 2. Create default tenant and app
INSERT INTO tenants (id, name) VALUES ('00000000-0000-0000-0000-000000000001', 'Default Tenant');
INSERT INTO applications (id, tenant_id, name, description) VALUES ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Default App', 'Migrated from single-tenant');

-- 3. Add app_id to existing tables (nullable first)
ALTER TABLE users ADD COLUMN app_id UUID;
ALTER TABLE social_accounts ADD COLUMN app_id UUID;
ALTER TABLE activity_logs ADD COLUMN app_id UUID;

-- 4. Migrate existing data
UPDATE users SET app_id = '00000000-0000-0000-0000-000000000001' WHERE app_id IS NULL;
UPDATE social_accounts SET app_id = '00000000-0000-0000-0000-000000000001' WHERE app_id IS NULL;
UPDATE activity_logs SET app_id = '00000000-0000-0000-0000-000000000001' WHERE app_id IS NULL;

-- 5. Add constraints
ALTER TABLE users ALTER COLUMN app_id SET NOT NULL;
ALTER TABLE social_accounts ALTER COLUMN app_id SET NOT NULL;
ALTER TABLE activity_logs ALTER COLUMN app_id SET NOT NULL;

ALTER TABLE users ADD CONSTRAINT fk_users_app FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE social_accounts ADD CONSTRAINT fk_social_accounts_app FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE activity_logs ADD CONSTRAINT fk_activity_logs_app FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;

-- Update indexes
-- Attempt to drop the existing unique index on email if it exists. 
-- Note: The name might vary, but 'idx_users_email' is standard GORM.
DROP INDEX IF EXISTS idx_users_email; 
CREATE UNIQUE INDEX idx_email_app_id ON users(email, app_id);

CREATE INDEX idx_social_accounts_app_id ON social_accounts(app_id);
CREATE INDEX idx_activity_logs_app_id ON activity_logs(app_id);

-- 6. Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success) 
VALUES ('20260105_add_multi_tenancy', 'Add Multi-Tenancy Support', NOW(), true)
ON CONFLICT (version) DO NOTHING;
