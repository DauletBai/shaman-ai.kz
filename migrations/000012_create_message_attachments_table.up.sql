-- migrations/000012_create_message_attachments_table.up.sql
CREATE TABLE IF NOT EXISTS message_attachments (
    id INT PRIMARY KEY AUTO_INCREMENT,
    dialogue_id INT NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    server_path VARCHAR(512) NOT NULL,
    url VARCHAR(512) NULL,
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (dialogue_id) REFERENCES dialogues(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;