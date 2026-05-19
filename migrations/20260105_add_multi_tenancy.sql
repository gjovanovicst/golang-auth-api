-- 1. Create new tables (idempotent)
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS oauth_provider_configs (
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_provider ON oauth_provider_configs(app_id, provider);

-- 2. Create default tenant and app (idempotent)
INSERT INTO tenants (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Tenant')
ON CONFLICT (id) DO NOTHING;

INSERT INTO applications (id, tenant_id, name, description)
VALUES ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Default App', 'Migrated from single-tenant')
ON CONFLICT (id) DO NOTHING;

-- 3. Add app_id to existing tables (nullable first, idempotent)
ALTER TABLE users ADD COLUMN IF NOT EXISTS app_id UUID;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS app_id UUID;
ALTER TABLE activity_logs ADD COLUMN IF NOT EXISTS app_id UUID;

-- 4. Migrate existing data
UPDATE users SET app_id = '00000000-0000-0000-0000-000000000001' WHERE app_id IS NULL;
UPDATE social_accounts SET app_id = '00000000-0000-0000-0000-000000000001' WHERE app_id IS NULL;
UPDATE activity_logs SET app_id = '00000000-0000-0000-0000-000000000001' WHERE app_id IS NULL;

-- 5. Make app_id NOT NULL (only if currently nullable)
DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'app_id' AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE users ALTER COLUMN app_id SET NOT NULL;
    END IF;
END $$;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'social_accounts' AND column_name = 'app_id' AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE social_accounts ALTER COLUMN app_id SET NOT NULL;
    END IF;
END $$;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'activity_logs' AND column_name = 'app_id' AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE activity_logs ALTER COLUMN app_id SET NOT NULL;
    END IF;
END $$;

-- 5b. Add foreign key constraints (only if they don't already exist)
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_users_app'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT fk_users_app
            FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_social_accounts_app'
    ) THEN
        ALTER TABLE social_accounts ADD CONSTRAINT fk_social_accounts_app
            FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_activity_logs_app'
    ) THEN
        ALTER TABLE activity_logs ADD CONSTRAINT fk_activity_logs_app
            FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;
    END IF;
END $$;

-- 6. Update indexes (idempotent)
DROP INDEX IF EXISTS idx_users_email;
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_app_id ON users(email, app_id);
CREATE INDEX IF NOT EXISTS idx_social_accounts_app_id ON social_accounts(app_id);
CREATE INDEX IF NOT EXISTS idx_activity_logs_app_id ON activity_logs(app_id);

-- 7. Record migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260105_add_multi_tenancy', 'Add Multi-Tenancy Support', NOW(), true)
ON CONFLICT (version) DO NOTHING;
