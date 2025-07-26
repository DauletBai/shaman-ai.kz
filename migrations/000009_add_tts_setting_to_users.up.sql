ALTER TABLE users ADD COLUMN tts_enabled_default BOOLEAN DEFAULT TRUE;
-- Опционально: обновить существующих пользователей, если значение по умолчанию должно быть TRUE
UPDATE users SET tts_enabled_default = TRUE WHERE tts_enabled_default IS NULL;