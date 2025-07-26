-- migrations/000011_add_email_verification_to_users.up.sql
ALTER TABLE users
ADD COLUMN email_verification_token VARCHAR(100) NULL,
ADD COLUMN email_verification_token_expires_at TIMESTAMP NULL,
ADD COLUMN is_email_verified BOOLEAN DEFAULT FALSE NOT NULL,
ADD COLUMN email_verified_at TIMESTAMP NULL;

CREATE INDEX idx_users_email_verification_token ON users (email_verification_token);