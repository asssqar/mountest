-- Видимость варианта в публичном каталоге.
-- Скрытый вариант не появляется в списках и нельзя начать по нему новую попытку.
-- Уже начатые попытки и их результаты остаются доступны (по токену).
-- Существующие варианты получают TRUE — поведение не меняется на лету.
ALTER TABLE variants
    ADD COLUMN IF NOT EXISTS is_published BOOLEAN NOT NULL DEFAULT TRUE;

-- Частичный индекс под публичный листинг — быстрый WHERE v.is_published.
CREATE INDEX IF NOT EXISTS variants_published_subject_idx
    ON variants(subject_id) WHERE is_published;
