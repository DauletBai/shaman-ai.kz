-- migrations/000003_add_role_id_to_users.up.sql
ALTER TABLE users ADD COLUMN role_id INT,
ADD CONSTRAINT fk_users_role_id
    FOREIGN KEY (role_id) REFERENCES roles(id)
    ON DELETE SET NULL;

-- Обновление существующих пользователей
-- Убедись, что роль 'user' существует перед выполнением этого
UPDATE users u
JOIN roles r ON r.name = 'user'
SET u.role_id = r.id
WHERE u.role_id IS NULL;