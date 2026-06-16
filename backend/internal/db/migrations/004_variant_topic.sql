-- Необязательная тема варианта (отображается под названием теста).
ALTER TABLE variants
    ADD COLUMN IF NOT EXISTS topic TEXT;
