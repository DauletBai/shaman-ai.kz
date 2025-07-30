-- migrations/000015_add_gateway_fields_to_payments.down.sql
ALTER TABLE payments
DROP COLUMN gateway_order_id,
DROP COLUMN gateway_name;
