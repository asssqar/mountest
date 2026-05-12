-- Roles for admins (superadmin vs editor) and per-variant ownership.
-- Idempotent: миграция гоняется на каждом старте, поэтому все шаги через IF NOT EXISTS / DO-блоки.

ALTER TABLE admins
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'editor';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'admins_role_check'
    ) THEN
        ALTER TABLE admins
            ADD CONSTRAINT admins_role_check
            CHECK (role IN ('superadmin', 'editor'));
    END IF;
END$$;

UPDATE admins
SET role = 'superadmin'
WHERE username = 'admin' AND role <> 'superadmin';

ALTER TABLE variants
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES admins(id) ON DELETE SET NULL;

UPDATE variants
SET created_by = (SELECT id FROM admins WHERE role = 'superadmin' ORDER BY created_at LIMIT 1)
WHERE created_by IS NULL;

CREATE INDEX IF NOT EXISTS variants_created_by_idx ON variants(created_by);
