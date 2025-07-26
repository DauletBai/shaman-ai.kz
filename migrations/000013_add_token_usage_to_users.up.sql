-- migrations/000013_add_token_usage_to_users.up.sql
ALTER TABLE users
ADD COLUMN tokens_used_input_this_period INT NOT NULL DEFAULT 0,
ADD COLUMN tokens_used_output_this_period INT NOT NULL DEFAULT 0,
ADD COLUMN billing_cycle_anchor_date TIMESTAMP NULL;

-- Для существующих пользователей с активной подпиской установим
-- дату начала текущего цикла равной дате начала подписки или текущей дате.
UPDATE users SET billing_cycle_anchor_date = IFNULL(subscription_start_date, NOW())
WHERE subscription_status = 'active';