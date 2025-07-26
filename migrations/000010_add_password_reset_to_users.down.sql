-- migrations/0000010_add_password_reset_to_users.down.sql
ALTER TABLE users
DROP INDEX idx_users_password_reset_token,
DROP COLUMN password_reset_token_expires_at,
DROP COLUMN password_reset_token;