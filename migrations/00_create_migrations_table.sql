-- Migration: Create schema_migrations tracking table
-- Date: 2024-01-03
-- Description: Creates a table to track which migrations have been applied

BEGIN;

-- Create migrations tracking table
CREATE TABLE IF NOT EXISTS schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    execution_time_ms INTEGER,
    success BOOLEAN NOT NULL DEFAULT true,
    error_message TEXT,
    checksum VARCHAR(64)
);

-- Add indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_schema_migrations_version ON schema_migrations(version);
CREATE INDEX IF NOT EXISTS idx_schema_migrations_applied_at ON schema_migrations(applied_at);

-- Add comments
COMMENT ON TABLE schema_migrations IS 'Tracks which database migrations have been applied';
COMMENT ON COLUMN schema_migrations.version IS 'Migration version (YYYYMMDD_HHMMSS format)';
COMMENT ON COLUMN schema_migrations.name IS 'Migration name/description';
COMMENT ON COLUMN schema_migrations.applied_at IS 'When the migration was applied';
COMMENT ON COLUMN schema_migrations.execution_time_ms IS 'How long the migration took to execute';
COMMENT ON COLUMN schema_migrations.success IS 'Whether the migration succeeded';
COMMENT ON COLUMN schema_migrations.error_message IS 'Error message if migration failed';
COMMENT ON COLUMN schema_migrations.checksum IS 'SHA256 checksum of migration file content';

-- Insert initial migration record (this migration itself)
INSERT INTO schema_migrations (version, name, success, execution_time_ms)
VALUES ('00000000_000000', 'create_migrations_table', true, 0)
ON CONFLICT (version) DO NOTHING;

COMMIT;

