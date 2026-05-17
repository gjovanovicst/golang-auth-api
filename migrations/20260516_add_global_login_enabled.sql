-- Add global_login_enabled flag to applications table.
-- When true, users can log in to this app using credentials registered in any app
-- on the platform (single identity across all apps). Defaults to false (opt-in).
ALTER TABLE applications ADD COLUMN IF NOT EXISTS global_login_enabled BOOLEAN NOT NULL DEFAULT false;
