// internal/auth/password.go
package auth

import (
	"golang.org/x/crypto/bcrypt"
	"regexp"
	"unicode"
    "strings"
    "time"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil 
}

func IsPasswordComplex(password string) bool {
	if len(password) < 8 {
		return false
	}
	var (
		hasLetter bool
		hasDigit  bool
		hasSymbol bool
	)
	for _, char := range password {
		switch {
		case unicode.IsLetter(char):
			hasLetter = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSymbol = true
		}
	}
	return hasLetter && hasDigit && hasSymbol
}

var nonAlphaSpaceDash = regexp.MustCompile(`[^\p{L}\s-]`)

func SanitizeName(name string) string {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) == 0 {
		return ""
	}
	// cleaned := CollapseWhitespace(nonAlphaSpaceDash.ReplaceAllString(trimmed, "")) // Доп. функцией
	cleaned := nonAlphaSpaceDash.ReplaceAllString(trimmed, "") 

	if len(cleaned) == 0 {
		return ""
	}
	r := []rune(cleaned)
	r[0] = unicode.ToUpper(r[0])

	// Опционально: можно привести остальные буквы к нижнему регистру,
	// но это может быть нежелательно для составных фамилий.
	// for i := 1; i < len(r); i++ {
	// 	r[i] = unicode.ToLower(r[i])
	// }
	return string(r)
}

func IsAdult(birthday string) bool {
	birthDate, err := time.Parse("2006-01-02", birthday)
	if err != nil {
		return false 
	}
	eighteenYearsAgo := time.Now().AddDate(-18, 0, 0)
	return !birthDate.After(eighteenYearsAgo)
}

// --- УЛУЧШЕННЫЙ ValidatePhone ---
// Требует формат +7 и 10 цифр после
var phoneRegex = regexp.MustCompile(`^\+7\d{10}$`)

func ValidatePhone(phone string) bool {
	// Можно добавить очистку от скобок, пробелов, дефисов перед проверкой, если нужно
	// cleanedPhone := ...
	// return phoneRegex.MatchString(cleanedPhone)
	return phoneRegex.MatchString(phone)
}