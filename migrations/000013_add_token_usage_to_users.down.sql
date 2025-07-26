-- migrations/000013_add_token_usage_to_users.down.sql
ALTER TABLE users
DROP COLUMN billing_cycle_anchor_date,
DROP COLUMN tokens_used_output_this_period,
DROP COLUMN tokens_used_input_this_period;