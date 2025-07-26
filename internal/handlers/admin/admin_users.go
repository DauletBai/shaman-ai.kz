// internal/handlers/admin/admin_users.go
package adminhandlers

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url" 
	"shaman-ai.kz/internal/auth" 
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/handlers"
	"shaman-ai.kz/internal/models"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/validation" 
	"strconv"
	"strings" 
)

const DefaultUsersPerPage = 10

// AdminUsersListPageHandler отображает список пользователей с пагинацией.
func AdminUsersListPageHandler(app *handlers.AppHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := app.NewPageData(r)
		data.AdminPageTitle = "Управление пользователями"

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		limit := DefaultUsersPerPage
		offset := (page - 1) * limit

		users, totalUsers, err := db.GetAllUsers(limit, offset)
		if err != nil {
			slog.Error("AdminUsersListPageHandler: не удалось получить пользователей", "error", err)
			http.Error(w, "Ошибка сервера при загрузке пользователей", http.StatusInternalServerError)
			return
		}

		data.Users = users
		data.TotalUsers = totalUsers
		data.CurrentPage = page
		data.Limit = limit
		data.TotalPages = int(math.Ceil(float64(totalUsers) / float64(limit)))

		app.RenderAdminPage(w, r, "users_list.html", data)
	}
}

// AdminEditUserPageHandler отображает страницу редактирования пользователя.
func AdminEditUserPageHandler(app *handlers.AppHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := app.NewPageData(r)
		userIDStr := r.URL.Query().Get("id")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil || userID == 0 {
			slog.Error("AdminEditUserPageHandler: неверный ID пользователя", "id_str", userIDStr, "error", err)
			http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
			return
		}

		user, err := db.GetUserByID(userID)
		if err != nil {
			slog.Error("AdminEditUserPageHandler: пользователь не найден", "userID", userID, "error", err)
			http.NotFound(w, r)
			return
		}
		data.EditingUser = user
		data.AdminPageTitle = fmt.Sprintf("Редактирование пользователя: %s", user.Email)
		data.FormAction = fmt.Sprintf("/admin/users/edit?id=%d", userID)

		allRoles, err := db.GetAllRoles()
		if err != nil {
			slog.Error("AdminEditUserPageHandler: не удалось получить список ролей", "error", err)
			// Можно продолжить без ролей или показать ошибку
		}
		data.AllRoles = allRoles

		// Дополнительно можно загрузить информацию о подписке пользователя, если нужно ее редактировать
		// ...

		app.RenderAdminPage(w, r, "user_edit.html", data)
	}
}

// AdminUpdateUserHandler обрабатывает обновление данных пользователя.
func AdminUpdateUserHandler(app *handlers.AppHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			slog.Error("AdminUpdateUserHandler: ошибка парсинга формы", "error", err)
			app.SessionManager.Put(r.Context(), "flash_error", "Ошибка сервера: не удалось обработать форму.")
			http.Redirect(w, r, "/admin/users", http.StatusSeeOther) 
			return
		}

		userIDStr := r.FormValue("userID")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil || userID == 0 {
			slog.Error("AdminUpdateUserHandler: неверный userID из формы", "userID_str", userIDStr, "error", err)
			app.SessionManager.Put(r.Context(), "flash_error", "Ошибка: Неверный ID пользователя.")
			http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
			return
		}

		// Получаем текущего пользователя для передачи в шаблон в случае ошибки
		editingUser, errUser := db.GetUserByID(userID)
		if errUser != nil {
			slog.Error("AdminUpdateUserHandler: не удалось получить пользователя для редактирования", "targetUserID", userID, "error", errUser)
			app.SessionManager.Put(r.Context(), "flash_error", "Ошибка: Пользователь для редактирования не найден.")
			http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
			return
		}
		
		// Сбор данных из формы
		firstName := strings.TrimSpace(r.PostForm.Get("first_name"))
		lastName := strings.TrimSpace(r.PostForm.Get("last_name"))
		phone := strings.TrimSpace(r.PostForm.Get("phone"))
		roleIDStr := r.FormValue("role_id")
		ttsEnabled := r.PostForm.Get("tts_enabled_default") == "on"


		// Валидация
		validationErrors := url.Values{}
		if firstName == "" {
			validationErrors.Add("first_name", "Имя не может быть пустым.")
		} else if !validation.ValidateAlphaSpace(firstName) { // Пример использования кастомного валидатора, если он есть
            validationErrors.Add("first_name", "Имя может содержать только буквы, пробелы и дефисы.")
        }

		if lastName == "" {
			validationErrors.Add("last_name", "Фамилия не может быть пустой.")
		} else if !validation.ValidateAlphaSpace(lastName) {
            validationErrors.Add("last_name", "Фамилия может содержать только буквы, пробелы и дефисы.")
        }
        
		var phonePtr *string
		if phone != "" {
			if !auth.ValidatePhone(phone) { // Используем валидатор из auth
				validationErrors.Add("phone", "Введите корректный номер телефона (например, +7XXXXXXXXXX).")
			} else {
				phonePtr = &phone
			}
		}

		newRoleID, errRole := strconv.ParseInt(roleIDStr, 10, 64)
		if errRole != nil {
			validationErrors.Add("role_id", "Неверный формат ID роли.")
		} else {
			// Дополнительная проверка, существует ли такая роль
			_, errGetRole := db.GetRoleByID(newRoleID)
			if errGetRole != nil {
				validationErrors.Add("role_id", "Выбрана несуществующая роль.")
			}
		}

		if len(validationErrors) > 0 {
			slog.Warn("AdminUpdateUserHandler: ошибки валидации при обновлении пользователя", "userID", userID, "errors", validationErrors)
			app.SessionManager.Put(r.Context(), "flash_error", "Пожалуйста, исправьте ошибки в форме.")
			
			pageData := app.NewPageData(r)
			pageData.AdminPageTitle = fmt.Sprintf("Редактирование пользователя (Ошибка): %s", editingUser.Email)
			pageData.EditingUser = editingUser
			pageData.Errors = validationErrors
			pageData.FormValues = r.PostForm // Передаем введенные значения обратно в форму
			allRoles, _ := db.GetAllRoles()
			pageData.AllRoles = allRoles
			pageData.FormAction = fmt.Sprintf("/admin/users/update?id=%d", userID) // или просто /admin/users/update, если ID в скрытом поле

			app.RenderAdminPage(w, r, "user_edit.html", pageData)
			return
		}

		// Подготовка данных для обновления в БД
		updateData := db.AdminUpdateUserData{
			FirstName:  auth.SanitizeName(firstName),
			LastName:   auth.SanitizeName(lastName),
			Phone:      phonePtr,
			RoleID:     newRoleID,
			TTSEnabledDefault: ttsEnabled,
		}

		err = db.UpdateUserByAdmin(userID, updateData)
		if err != nil {
			slog.Error("AdminUpdateUserHandler: не удалось обновить данные пользователя", "userID", userID, "error", err)
			app.SessionManager.Put(r.Context(), "flash_error", fmt.Sprintf("Ошибка обновления данных пользователя: %s", err.Error()))
			// Можно снова отрендерить страницу редактирования с ошибкой
			pageData := app.NewPageData(r)
			pageData.AdminPageTitle = fmt.Sprintf("Редактирование пользователя (Ошибка): %s", editingUser.Email)
			pageData.EditingUser = editingUser
            pageData.Errors = url.Values{"general": {err.Error()}} 
			pageData.FormValues = r.PostForm
			allRoles, _ := db.GetAllRoles()
			pageData.AllRoles = allRoles
			pageData.FormAction = fmt.Sprintf("/admin/users/update?id=%d", userID)
			app.RenderAdminPage(w, r, "user_edit.html", pageData)
			return
		}

		slog.Info("Данные пользователя успешно обновлены админом", 
			"adminUserID", r.Context().Value(middleware.UserContextKey).(*models.User).ID, 
			"targetUserID", userID)
		app.SessionManager.Put(r.Context(), "flash_success", "Данные пользователя успешно обновлены.")
		http.Redirect(w, r, fmt.Sprintf("/admin/users/edit?id=%d", userID), http.StatusSeeOther) 
        // Или на список пользователей: http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}