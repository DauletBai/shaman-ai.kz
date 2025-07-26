// internal/db/users.go
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"shaman-ai.kz/internal/models"
	"strings"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/go-sql-driver/mysql"
)

// AdminUpdateUserData содержит поля, которые администратор может обновить.
type AdminUpdateUserData struct {
	FirstName         string
	LastName          string
	Phone             *string
	RoleID            int64
	TTSEnabledDefault bool
}

// UpdateUserByAdmin обновляет данные пользователя администратором.
func UpdateUserByAdmin(userID int64, data AdminUpdateUserData) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}

	_, err := GetRoleByID(data.RoleID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования роли ID %d: %w", data.RoleID, err)
	}

	query := `UPDATE users SET
                first_name = ?,
                last_name = ?,
                phone = ?,
                role_id = ?,
                tts_enabled_default = ?,
                updated_at = ?
              WHERE id = ?`

	var phoneSQL sql.NullString
	if data.Phone != nil {
		phoneSQL.String = *data.Phone
		phoneSQL.Valid = true
	}

	_, err = DB.Exec(query,
		data.FirstName,
		data.LastName,
		phoneSQL,
		data.RoleID,
		data.TTSEnabledDefault,
		time.Now(),
		userID,
	)

	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			if strings.Contains(strings.ToLower(mysqlErr.Message), "phone") {
				return errors.New("пользователь с таким телефоном уже существует")
			}
			return fmt.Errorf("нарушение ограничения уникальности при обновлении: %w", err)
		}
		slog.Error("Ошибка обновления данных пользователя админом", "userID", userID, "error", err)
		return fmt.Errorf("не удалось обновить данные пользователя: %w", err)
	}

	slog.Info("Данные пользователя обновлены админом", "userID", userID)
	return nil
}

// GenerateSecureToken генерирует безопасный случайный токен.
func GenerateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken хеширует токен для безопасного хранения.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func SetPasswordResetToken(userID int64, rawToken string) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	hashedToken := HashToken(rawToken)
	expiresAt := time.Now().Add(1 * time.Hour) // Токен действителен 1 час

	query := `UPDATE users SET password_reset_token = ?, password_reset_token_expires_at = ?, updated_at = ? WHERE id = ?`
	_, err := DB.Exec(query, hashedToken, expiresAt, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка установки токена сброса пароля", "userID", userID, "error", err)
		return fmt.Errorf("не удалось установить токен сброса пароля: %w", err)
	}
	slog.Info("Токен сброса пароля установлен", "userID", userID)
	return nil
}

func GetUserByPasswordResetToken(rawToken string) (*models.User, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	hashedToken := HashToken(rawToken)
	row := DB.QueryRow(getFullUserQuery()+" WHERE u.password_reset_token = ?", hashedToken)
	return scanFullUser(row)
}

func ClearPasswordResetToken(userID int64) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	query := `UPDATE users SET password_reset_token = NULL, password_reset_token_expires_at = NULL, updated_at = ? WHERE id = ?`
	_, err := DB.Exec(query, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка очистки токена сброса пароля", "userID", userID, "error", err)
		return fmt.Errorf("не удалось очистить токен сброса пароля: %w", err)
	}
	return nil
}

func SetEmailVerificationToken(userID int64, rawToken string) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	hashedToken := HashToken(rawToken)
	expiresAt := time.Now().Add(24 * time.Hour)

	query := `UPDATE users SET
                email_verification_token = ?,
                email_verification_token_expires_at = ?,
                updated_at = ?
              WHERE id = ? AND is_email_verified = FALSE`
	res, err := DB.Exec(query, hashedToken, expiresAt, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка установки токена верификации email", "userID", userID, "error", err)
		return fmt.Errorf("не удалось установить токен верификации email: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		slog.Warn("Токен верификации email не установлен (возможно, email уже верифицирован или пользователь не найден)", "userID", userID)
		existingUser, _ := GetUserByID(userID)
		if existingUser != nil && existingUser.IsEmailVerified {
			return nil
		}
		return fmt.Errorf("пользователь не найден или email уже верифицирован")
	}
	slog.Info("Токен верификации email установлен", "userID", userID)
	return nil
}

func VerifyUserEmail(rawToken string) (int64, error) {
	if DB == nil {
		return 0, errors.New("база данных не инициализирована")
	}
	hashedToken := HashToken(rawToken)

	var userID int64
	var dbTokenExpiresAt sql.NullTime
	var dbIsEmailVerified bool

	querySelect := `SELECT id, email_verification_token_expires_at, is_email_verified
	                 FROM users
	                 WHERE email_verification_token = ?`
	err := DB.QueryRow(querySelect, hashedToken).Scan(&userID, &dbTokenExpiresAt, &dbIsEmailVerified)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Warn("Неверный или уже использованный токен верификации email", "token_hash_prefix", hashedToken[:10])
			return 0, errors.New("неверная или истекшая ссылка для верификации")
		}
		slog.Error("Ошибка поиска пользователя по токену верификации email", "error", err)
		return 0, fmt.Errorf("ошибка сервера при проверке токена: %w", err)
	}

	if dbIsEmailVerified {
		slog.Info("Попытка верификации уже верифицированного email", "userID", userID)
		return userID, errors.New("email уже подтвержден")
	}

	if !dbTokenExpiresAt.Valid || time.Now().After(dbTokenExpiresAt.Time) {
		slog.Warn("Истек срок действия токена верификации email", "userID", userID)
		_, errClear := DB.Exec("UPDATE users SET email_verification_token = NULL, email_verification_token_expires_at = NULL WHERE id = ?", userID)
		if errClear != nil {
			slog.Error("Ошибка очистки просроченного токена верификации", "userID", userID, "error", errClear)
		}
		return 0, errors.New("срок действия ссылки для верификации истек")
	}

	queryUpdate := `UPDATE users SET
	                    is_email_verified = TRUE,
	                    email_verified_at = ?,
	                    email_verification_token = NULL,
	                    email_verification_token_expires_at = NULL,
	                    updated_at = ?
	                WHERE id = ? AND email_verification_token = ?`

	now := time.Now()
	res, err := DB.Exec(queryUpdate, now, now, userID, hashedToken)
	if err != nil {
		slog.Error("Ошибка обновления статуса верификации email", "userID", userID, "error", err)
		return 0, fmt.Errorf("не удалось верифицировать email: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		slog.Error("Верификация email не удалась (возможно, токен был изменен или использован)", "userID", userID)
		return 0, errors.New("не удалось верифицировать email, возможно, ссылка устарела")
	}

	slog.Info("Email успешно верифицирован", "userID", userID)
	return userID, nil
}

func CreateUser(user *models.User, defaultRoleName string) (int64, error) {
	if DB == nil {
		return 0, errors.New("база данных не инициализирована")
	}

	defaultRole, err := GetRoleByName(defaultRoleName)
	if err != nil {
		slog.Error("Не удалось получить роль по умолчанию для нового пользователя", "roleName", defaultRoleName, "error", err)
		return 0, fmt.Errorf("критическая ошибка: роль по умолчанию '%s' не найдена: %w", defaultRoleName, err)
	}
	if defaultRole == nil {
		return 0, fmt.Errorf("критическая ошибка: роль по умолчанию '%s' не найдена (nil)", defaultRoleName)
	}

	ttsEnabledDefaultValue := true
	if user.TTSEnabledDefault != nil {
		ttsEnabledDefaultValue = *user.TTSEnabledDefault
	}

	query := `INSERT INTO users (email, phone, password_hash, first_name, last_name, gender, birthday, role_id, created_at, updated_at, subscription_status, tts_enabled_default)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	initialSubscriptionStatus := models.SubscriptionStatusInactive
	if user.SubscriptionStatus != "" {
		initialSubscriptionStatus = user.SubscriptionStatus
	}

	res, err := DB.Exec(query,
		user.Email,
		user.Phone,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.Gender,
		user.Birthday,
		defaultRole.ID,
		now,
		now,
		initialSubscriptionStatus,
		ttsEnabledDefaultValue,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			if strings.Contains(strings.ToLower(mysqlErr.Message), "email") {
				return 0, errors.New("пользователь с таким email уже существует")
			}
			if strings.Contains(strings.ToLower(mysqlErr.Message), "phone") {
				return 0, errors.New("пользователь с таким телефоном уже существует")
			}
			return 0, fmt.Errorf("нарушение ограничения уникальности: %w", err)
		}
		slog.Error("Ошибка при создании пользователя", "error", err, "email", user.Email)
		return 0, fmt.Errorf("не удалось создать пользователя: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		slog.Error("Ошибка при получении ID созданного пользователя", "error", err)
		return 0, fmt.Errorf("не удалось получить ID пользователя: %w", err)
	}

	slog.Info("Пользователь успешно создан", "user_id", id, "email", user.Email, "role_id", defaultRole.ID)
	return id, nil
}

func GetUserByEmail(email string) (*models.User, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	row := DB.QueryRow(getFullUserQuery()+" WHERE LOWER(u.email) = LOWER(?)", strings.ToLower(email))
	return scanFullUser(row)
}

func GetUserByID(id int64) (*models.User, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	row := DB.QueryRow(getFullUserQuery()+" WHERE u.id = ?", id)
	return scanFullUser(row)
}

func SetUserRole(userID int64, roleID int64) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	_, err := GetRoleByID(roleID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования роли ID %d: %w", roleID, err)
	}

	query := `UPDATE users SET role_id = ?, updated_at = ? WHERE id = ?`
	_, err = DB.Exec(query, roleID, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка обновления роли пользователя", "userID", userID, "roleID", roleID, "error", err)
		return fmt.Errorf("не удалось обновить роль пользователя: %w", err)
	}
	slog.Info("Роль пользователя обновлена", "userID", userID, "new_role_id", roleID)
	return nil
}

func UpdateUserSubscriptionDetails(userID int64, gatewaySubscriptionID, gatewayCustomerID string, status models.SubscriptionStatus, startDate, endDate, currentPeriodEnd time.Time) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	// При обновлении подписки также сбрасываем счетчики токенов и устанавливаем новую дату начала цикла
	query := `UPDATE users SET
				subscription_id = ?,
				customer_id = ?,
				subscription_status = ?,
				subscription_start_date = ?,
				subscription_end_date = ?,
				current_period_end = ?,
				tokens_used_input_this_period = 0,
				tokens_used_output_this_period = 0,
				billing_cycle_anchor_date = ?,
				updated_at = ?
			  WHERE id = ?`
	now := time.Now()
	_, err := DB.Exec(query,
		sql.NullString{String: gatewaySubscriptionID, Valid: gatewaySubscriptionID != ""},
		sql.NullString{String: gatewayCustomerID, Valid: gatewayCustomerID != ""},
		status,
		sql.NullTime{Time: startDate, Valid: !startDate.IsZero()},
		sql.NullTime{Time: endDate, Valid: !endDate.IsZero()},
		sql.NullTime{Time: currentPeriodEnd, Valid: !currentPeriodEnd.IsZero()},
		now, // billing_cycle_anchor_date
		now, // updated_at
		userID)
	if err != nil {
		slog.Error("Ошибка обновления данных подписки пользователя", "userID", userID, "error", err)
		return fmt.Errorf("не удалось обновить данные подписки: %w", err)
	}
	slog.Info("Данные подписки пользователя обновлены", "userID", userID, "status", status)
	return nil
}

func GetUserSubscriptionStatus(userID int64) (models.SubscriptionStatus, *time.Time, error) {
	if DB == nil {
		return models.SubscriptionStatusInactive, nil, errors.New("база данных не инициализирована")
	}
	var statusStr sql.NullString
	var currentPeriodEnd sql.NullTime
	query := `SELECT subscription_status, current_period_end FROM users WHERE id = ?`
	err := DB.QueryRow(query, userID).Scan(&statusStr, &currentPeriodEnd)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Warn("GetUserSubscriptionStatus: пользователь не найден", "userID", userID)
			return models.SubscriptionStatusInactive, nil, nil
		}
		slog.Error("Ошибка получения статуса подписки пользователя", "userID", userID, "error", err)
		return models.SubscriptionStatusInactive, nil, err
	}

	userStatus := models.SubscriptionStatusInactive
	if statusStr.Valid && statusStr.String != "" {
		userStatus = models.SubscriptionStatus(statusStr.String)
	}

	var periodEndPtr *time.Time
	if currentPeriodEnd.Valid {
		periodEndPtr = &currentPeriodEnd.Time
	}

	return userStatus, periodEndPtr, nil
}

func GetUserByGatewayCustomerID(customerID string) (*models.User, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	if customerID == "" {
		return nil, errors.New("customerID не может быть пустым")
	}
	row := DB.QueryRow(getFullUserQuery()+" WHERE u.customer_id = ?", customerID)
	return scanFullUser(row)
}

func UpdateUserSubscriptionPeriod(userID int64, newPeriodEnd time.Time, newStatus models.SubscriptionStatus) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}

	query := `UPDATE users SET current_period_end = ?, subscription_status = ?, updated_at = ? WHERE id = ?`
	_, err := DB.Exec(query, newPeriodEnd, newStatus, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка обновления периода подписки пользователя", "userID", userID, "newPeriodEnd", newPeriodEnd, "newStatus", newStatus, "error", err)
		return fmt.Errorf("не удалось обновить период подписки: %w", err)
	}
	slog.Info("Период подписки пользователя обновлен", "userID", userID, "newPeriodEnd", newPeriodEnd, "newStatus", newStatus)
	return nil
}

func GetAllUsers(limit, offset int) ([]*models.User, int, error) {
	if DB == nil {
		return nil, 0, errors.New("база данных не инициализирована")
	}

	countQuery := `SELECT COUNT(*) FROM users`
	var totalUsers int
	err := DB.QueryRow(countQuery).Scan(&totalUsers)
	if err != nil {
		slog.Error("Ошибка при подсчете общего количества пользователей", "error", err)
		return nil, 0, fmt.Errorf("ошибка подсчета пользователей: %w", err)
	}

	rows, err := DB.Query(getFullUserQuery()+" ORDER BY u.created_at DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		slog.Error("Ошибка при получении списка всех пользователей", "error", err)
		return nil, 0, fmt.Errorf("ошибка получения списка пользователей: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user, errScan := scanFullUser(rows)
		if errScan != nil {
			slog.Error("Ошибка сканирования пользователя при получении списка", "error", errScan)
			continue
		}
		users = append(users, user)
	}
	if err = rows.Err(); err != nil {
		slog.Error("Ошибка итерации по списку пользователей", "error", err)
		return nil, 0, fmt.Errorf("ошибка итерации по списку пользователей: %w", err)
	}

	return users, totalUsers, nil
}

func UpdateUserProfile(userID int64, firstName, lastName string, phone *string) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	query := `UPDATE users SET first_name = ?, last_name = ?, phone = ?, updated_at = ? WHERE id = ?`
	_, err := DB.Exec(query, firstName, lastName, phone, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка обновления профиля пользователя", "userID", userID, "error", err)
		return fmt.Errorf("не удалось обновить профиль пользователя: %w", err)
	}
	slog.Info("Профиль пользователя обновлен", "userID", userID)
	return nil
}

func UpdateUserPassword(userID int64, newPasswordHash string) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	query := `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`
	_, err := DB.Exec(query, newPasswordHash, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка обновления пароля пользователя", "userID", userID, "error", err)
		return fmt.Errorf("не удалось обновить пароль пользователя: %w", err)
	}
	slog.Info("Пароль пользователя обновлен", "userID", userID)
	return nil
}

func UpdateUserTTSEnabledDefault(userID int64, enabled bool) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}
	query := `UPDATE users SET tts_enabled_default = ?, updated_at = ? WHERE id = ?`
	_, err := DB.Exec(query, enabled, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка обновления настройки TTS пользователя", "userID", userID, "enabled", enabled, "error", err)
		return fmt.Errorf("не удалось обновить настройку TTS: %w", err)
	}
	slog.Info("Настройка TTS пользователя обновлена", "userID", userID, "enabled", enabled)
	return nil
}

// Новая функция для инкрементации счетчиков токенов
func IncrementTokenUsage(userID int64, inputTokens, outputTokens int) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	query := `UPDATE users SET
                tokens_used_input_this_period = tokens_used_input_this_period + ?,
                tokens_used_output_this_period = tokens_used_output_this_period + ?,
                updated_at = ?
              WHERE id = ?`
	_, err := DB.Exec(query, inputTokens, outputTokens, time.Now(), userID)
	if err != nil {
		slog.Error("Ошибка инкрементации использования токенов", "userID", userID, "error", err)
		return fmt.Errorf("не удалось обновить счетчик токенов: %w", err)
	}
	return nil
}


// --- Helper-функции для уменьшения дублирования кода ---

// getFullUserQuery возвращает SQL-запрос со всеми полями пользователя.
func getFullUserQuery() string {
	return `SELECT u.id, u.email, u.phone, u.password_hash, u.first_name, u.last_name, u.gender, u.birthday,
                   u.created_at, u.updated_at,
                   u.subscription_id, u.customer_id, u.subscription_status,
                   u.subscription_start_date, u.subscription_end_date, u.current_period_end,
                   u.role_id, r.name as role_name, u.tts_enabled_default,
                   u.is_email_verified, u.email_verified_at, u.password_reset_token, u.password_reset_token_expires_at,
                   u.tokens_used_input_this_period, u.tokens_used_output_this_period, u.billing_cycle_anchor_date
            FROM users u
            LEFT JOIN roles r ON u.role_id = r.id`
}

// scanner - это интерфейс, который удовлетворяется и *sql.Row, и *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanFullUser сканирует строку из БД в модель models.User.
func scanFullUser(row scanner) (*models.User, error) {
	user := &models.User{}
	var phone sql.NullString
	var subscriptionID, customerID, passwordResetToken sql.NullString
	var subscriptionStatus sql.NullString
	var subscriptionStartDate, subscriptionEndDate, currentPeriodEnd, emailVerifiedAt, passwordResetTokenExpiresAt, billingCycleAnchorDate sql.NullTime
	var roleID sql.NullInt64
	var roleName sql.NullString
	var ttsEnabledDefaultSQL sql.NullBool

	err := row.Scan(
		&user.ID, &user.Email, &phone, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.Gender, &user.Birthday,
		&user.CreatedAt, &user.UpdatedAt,
		&subscriptionID, &customerID, &subscriptionStatus,
		&subscriptionStartDate, &subscriptionEndDate, &currentPeriodEnd,
		&roleID, &roleName, &ttsEnabledDefaultSQL,
		&user.IsEmailVerified, &emailVerifiedAt, &passwordResetToken, &passwordResetTokenExpiresAt,
		&user.TokensUsedInputThisPeriod, &user.TokensUsedOutputThisPeriod, &billingCycleAnchorDate,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("ошибка сканирования данных пользователя: %w", err)
	}

	if phone.Valid {
		user.Phone = &phone.String
	}
	if subscriptionID.Valid {
		user.SubscriptionID = &subscriptionID.String
	}
	if customerID.Valid {
		user.CustomerID = &customerID.String
	}
	if subscriptionStatus.Valid && subscriptionStatus.String != "" {
		user.SubscriptionStatus = models.SubscriptionStatus(subscriptionStatus.String)
	} else {
		user.SubscriptionStatus = models.SubscriptionStatusInactive
	}
	if subscriptionStartDate.Valid {
		user.SubscriptionStartDate = &subscriptionStartDate.Time
	}
	if subscriptionEndDate.Valid {
		user.SubscriptionEndDate = &subscriptionEndDate.Time
	}
	if currentPeriodEnd.Valid {
		user.CurrentPeriodEnd = &currentPeriodEnd.Time
	}
	if roleID.Valid {
		user.RoleID = &roleID.Int64
	}
	if roleName.Valid {
		user.RoleName = &roleName.String
	}
	if ttsEnabledDefaultSQL.Valid {
		user.TTSEnabledDefault = &ttsEnabledDefaultSQL.Bool
	} else {
		defaultValue := true
		user.TTSEnabledDefault = &defaultValue
	}
	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}
	if passwordResetToken.Valid {
		user.PasswordResetToken = &passwordResetToken.String
	}
	if passwordResetTokenExpiresAt.Valid {
		user.PasswordResetTokenExpiresAt = &passwordResetTokenExpiresAt.Time
	}
	if billingCycleAnchorDate.Valid {
		user.BillingCycleAnchorDate = &billingCycleAnchorDate.Time
	}

	return user, nil
}