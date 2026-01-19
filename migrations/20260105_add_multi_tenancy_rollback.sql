ALTER TABLE users DROP CONSTRAINT fk_users_app;
ALTER TABLE social_accounts DROP CONSTRAINT fk_social_accounts_app;
ALTER TABLE activity_logs DROP CONSTRAINT fk_activity_logs_app;

DROP INDEX IF EXISTS idx_email_app_id;
-- Restore original index (assuming it was named idx_users_email)
CREATE UNIQUE INDEX idx_users_email ON users(email);

ALTER TABLE users DROP COLUMN app_id;
ALTER TABLE social_accounts DROP COLUMN app_id;
ALTER TABLE activity_logs DROP COLUMN app_id;

DROP TABLE oauth_provider_configs;
DROP TABLE applications;
DROP TABLE tenants;
