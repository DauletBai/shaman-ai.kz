// internal/email/sender.go
package email

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"os"
	"path/filepath"
	"runtime" 
	"shaman-ai.kz/internal/config"
	"strings" 
)

// SendEmail отправляет письмо.
// bodyIsHTML указывает, является ли тело письма HTML-кодом.
// templateName - имя файла шаблона (без пути, например "verification_email.html")
// templateData - данные для шаблона
func SendEmail(appCfg *config.Config, to, subject, bodyContent string, bodyIsHTML bool, templateName string, templateData interface{}) error {
	if appCfg.Email.SMTPhost == "" || appCfg.Email.Sender == "" {
		slog.Warn("SMTP хост или отправитель не настроены. Псевдо-отправка email.", "to", to, "subject", subject)
		slog.Debug("Тело письма (для псевдо-отправки)", "body", bodyContent, "template", templateName, "templateData", templateData)
		if appCfg.AppEnv != "development" {
			return fmt.Errorf("SMTP хост или отправитель не настроены для отправки email")
		}
		return nil
	}

	auth := smtp.PlainAuth("", appCfg.Email.SMTPuser, appCfg.Email.SMTPpassword, appCfg.Email.SMTPhost)
	addr := fmt.Sprintf("%s:%d", appCfg.Email.SMTPhost, appCfg.Email.SMTPport)

	var finalBody bytes.Buffer
	finalContentType := "text/plain; charset=\"UTF-8\""

	if bodyIsHTML && templateName != "" {
		_, currentFilePath, _, ok := runtime.Caller(0)
		var basePath string
		if ok {
			projectRoot := filepath.Join(filepath.Dir(currentFilePath), "..", "..")
			basePath = filepath.Join(projectRoot, "templates", "emails")
		} else {
			slog.Error("Не удалось определить путь к файлу sender.go для поиска email шаблонов, используется относительный путь 'templates/emails'")
			basePath = filepath.Join("templates", "emails") // Fallback
		}
		tplPath := filepath.Join(basePath, templateName)

		slog.Debug("Попытка загрузки HTML шаблона письма", "path", tplPath)
		if _, errStat := os.Stat(tplPath); os.IsNotExist(errStat) {
			slog.Error("HTML шаблон письма не найден", "path", tplPath, "error", errStat)
			finalBody.WriteString(bodyContent)
		} else {
			tpl, err := template.New(filepath.Base(tplPath)).ParseFiles(tplPath) // Используем New для имени шаблона
			if err != nil {
				slog.Error("Ошибка парсинга HTML шаблона письма", "template", templateName, "path", tplPath, "error", err)
				finalBody.WriteString(bodyContent)
			} else {
				err = tpl.Execute(&finalBody, templateData)
				if err != nil {
					slog.Error("Ошибка выполнения HTML шаблона письма", "template", templateName, "error", err)
					finalBody.Reset()
					finalBody.WriteString(bodyContent)
				} else {
					finalContentType = "text/html; charset=\"UTF-8\""
				}
			}
		}
	} else {
		finalBody.WriteString(bodyContent)
	}

	headers := make(map[string]string)
	headers["From"] = appCfg.Email.Sender
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-version"] = "1.0"
	headers["Content-Type"] = finalContentType

	var msgBuilder strings.Builder
	for k, v := range headers {
		msgBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString(finalBody.String())

	err := smtp.SendMail(addr, auth, appCfg.Email.Sender, []string{to}, []byte(msgBuilder.String()))
	if err != nil {
		slog.Error("Ошибка отправки email", "to", to, "error", err)
		return fmt.Errorf("не удалось отправить email: %w", err)
	}

	slog.Info("Email успешно отправлен", "to", to, "subject", subject)
	return nil
}