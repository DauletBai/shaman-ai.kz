-- migrations/0000010_add_password_reset_to_users.up.sql
ALTER TABLE users
ADD COLUMN password_reset_token VARCHAR(100) NULL,
ADD COLUMN password_reset_token_expires_at TIMESTAMP NULL;

CREATE INDEX idx_users_password_reset_token ON users (password_reset_token);
