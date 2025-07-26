-- migrations/000003_add_role_id_to_users.down.sql
ALTER TABLE users DROP FOREIGN KEY fk_users_role_id, DROP COLUMN role_id;