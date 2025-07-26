-- migration: 000014_add_phone_verification_to_users.down.sql
ALTER TABLE users
DROP COLUMN is_phone_verified,
DROP COLUMN phone_verification_code,
DROP COLUMN phone_verification_code_expires_at;