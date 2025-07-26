// internal/sms/sender.go
package sms

import (
    "fmt"
    "log/slog"
    "net/http"
    "net/url"
    "shaman-ai.kz/internal/config"
    "strings"
)

// SendSMS отправляет SMS через API шлюза.
// Эта функция - ПРИМЕР, ее нужно будет адаптировать под API вашего конкретного провайдера.
func SendSMS(cfg *config.Config, phoneNumber, message string) error {
    if cfg.SMS.APIKey == "" { // Предполагается, что в cfg есть поле SMS типа SMSConfig
        slog.Warn("SMS шлюз не настроен. Псевдо-отправка SMS.", "to", phoneNumber, "message", message)
        return nil
    }

    // Пример для шлюза, который принимает параметры в URL
    data := url.Values{}
    data.Set("api_key", cfg.SMS.APIKey)
    data.Set("to", phoneNumber)
    data.Set("text", message)
    data.Set("from", cfg.SMS.SenderID)

    req, err := http.NewRequest("POST", cfg.SMS.APIURL, strings.NewReader(data.Encode()))
    if err != nil {
        slog.Error("Ошибка создания запроса к SMS шлюзу", "error", err)
        return err
    }
    req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        slog.Error("Ошибка отправки запроса к SMS шлюзу", "error", err)
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        slog.Error("SMS шлюз вернул ошибку", "status", resp.Status)
        return fmt.Errorf("ошибка отправки SMS: статус %d", resp.StatusCode)
    }

    slog.Info("SMS успешно отправлено", "to", phoneNumber)
    return nil
}