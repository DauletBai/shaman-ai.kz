// internal/db/roles_db.go
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"shaman-ai.kz/internal/models"
	"strings"
	"time"
	//"github.com/go-sql-driver/mysql"
)

// CreateRoleIfNotExists создает роль, если она еще не существует.
func CreateRoleIfNotExists(role *models.Role) (int64, error) {
	if DB == nil {
		return 0, errors.New("база данных не инициализирована")
	}
	existingRole, err := GetRoleByName(role.Name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("ошибка проверки существующей роли '%s': %w", role.Name, err)
	}
	if existingRole != nil {
		slog.Debug("Роль уже существует, пропуск создания", "role_name", role.Name, "role_id", existingRole.ID)
		return existingRole.ID, nil
	}

	query := `INSERT INTO roles (name, description, created_at, updated_at) VALUES (?, ?, ?, ?)`
	now := time.Now()
	res, err := DB.Exec(query, role.Name, role.Description, now, now)
	if err != nil {
		// Можно добавить более специфичную обработку для MariaDB, если нужно (например, mysqlErr.Number == 1062 для дубликата)
		// var mysqlErr *mysql.MySQLError
		// if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		//  slog.Warn("Попытка вставить дублирующуюся роль, хотя проверка GetRoleByName должна была это предотвратить.", "role_name", role.Name)
		//  // Повторно пытаемся получить ID, так как гонка состояний возможна
		//  retryRole, retryErr := GetRoleByName(role.Name)
		//  if retryErr == nil && retryRole != nil {
		//      return retryRole.ID, nil
		//  }
		//  return 0, fmt.Errorf("ошибка создания роли (дубликат, не удалось получить): %w", err)
		// }
		slog.Error("Ошибка при создании роли", "role_name", role.Name, "error", err)
		return 0, fmt.Errorf("не удалось создать роль '%s': %w", role.Name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		slog.Error("Ошибка при получении ID созданной роли", "role_name", role.Name, "error", err)
		return 0, fmt.Errorf("не удалось получить ID роли '%s': %w", role.Name, err)
	}
	slog.Info("Роль успешно создана", "role_id", id, "role_name", role.Name)
	return id, nil
}

// GetRoleByName возвращает роль по ее имени.
func GetRoleByName(name string) (*models.Role, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	query := `SELECT id, name, description, created_at, updated_at FROM roles WHERE LOWER(name) = LOWER(?)`
	row := DB.QueryRow(query, strings.ToLower(name))
	role := &models.Role{}
	var description sql.NullString
	err := row.Scan(&role.ID, &role.Name, &description, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err // Роль не найдена
		}
		slog.Error("Ошибка при поиске роли по имени", "name", name, "error", err)
		return nil, fmt.Errorf("ошибка получения роли по имени '%s': %w", name, err)
	}
	if description.Valid {
		role.Description = description.String
	}
	return role, nil
}

// GetRoleByID возвращает роль по ее ID.
func GetRoleByID(id int64) (*models.Role, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	query := `SELECT id, name, description, created_at, updated_at FROM roles WHERE id = ?`
	row := DB.QueryRow(query, id)
	role := &models.Role{}
	var description sql.NullString
	err := row.Scan(&role.ID, &role.Name, &description, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err // Роль не найдена
		}
		slog.Error("Ошибка при поиске роли по ID", "id", id, "error", err)
		return nil, fmt.Errorf("ошибка получения роли по ID %d: %w", id, err)
	}
	if description.Valid {
		role.Description = description.String
	}
	return role, nil
}

// GetAllRoles возвращает список всех ролей.
func GetAllRoles() ([]models.Role, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	query := `SELECT id, name, description, created_at, updated_at FROM roles ORDER BY name ASC`
	rows, err := DB.Query(query)
	if err != nil {
		slog.Error("Ошибка при получении списка всех ролей", "error", err)
		return nil, fmt.Errorf("ошибка получения списка ролей: %w", err)
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		var description sql.NullString
		if err := rows.Scan(&role.ID, &role.Name, &description, &role.CreatedAt, &role.UpdatedAt); err != nil {
			slog.Error("Ошибка сканирования роли при получении списка", "error", err)
			continue // Пропускаем ошибочную строку
		}
		if description.Valid {
			role.Description = description.String
		}
		roles = append(roles, role)
	}
	if err = rows.Err(); err != nil {
		slog.Error("Ошибка итерации по списку ролей", "error", err)
		return nil, fmt.Errorf("ошибка итерации по списку ролей: %w", err)
	}
	return roles, nil
}