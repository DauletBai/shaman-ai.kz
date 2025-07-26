-- migrations/000007_create_dialogues_table.up.sql
CREATE TABLE IF NOT EXISTS dialogues (
    id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,
    chat_session_uuid VARCHAR(36) NOT NULL,
    user_prompt TEXT NOT NULL,
    ai_response TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (chat_session_uuid) REFERENCES chat_sessions(uuid) ON DELETE CASCADE,
    INDEX idx_dialogues_chat_session_uuid_created_at (chat_session_uuid, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;