-- migrations/000015_add_gateway_fields_to_payments.up.sql
ALTER TABLE payments
ADD COLUMN gateway_order_id VARCHAR(255),
ADD COLUMN gateway_name VARCHAR(50) DEFAULT 'none';

COMMENT ON COLUMN payments.gateway_order_id IS 'ID заказа во внешней платежной системе';
COMMENT ON COLUMN payments.gateway_name IS 'Название платежного шлюза, например, bcc';