-- migrations/000011_add_email_verification_to_users.down.sql
ALTER TABLE users
DROP INDEX idx_users_email_verification_token,
DROP COLUMN email_verified_at,
DROP COLUMN is_email_verified,
DROP COLUMN email_verification_token_expires_at,
DROP COLUMN email_verification_token;