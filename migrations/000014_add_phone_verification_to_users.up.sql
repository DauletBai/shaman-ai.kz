-- migrations/000014_add_phone_verification_to_users.up.sql
ALTER TABLE users
ADD COLUMN is_phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN phone_verification_code VARCHAR(10) DEFAULT NULL,
ADD COLUMN phone_verification_code_expires_at TIMESTAMP NULL DEFAULT NULL;

CREATE INDEX idx_users_phone_verification_code ON users (phone_verification_code);