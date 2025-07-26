// internal/validation/validation.go
package validation

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	//"unicode"

	"github.com/go-playground/validator/v10"

	"shaman-ai.kz/internal/auth" 
)

var validate *validator.Validate
var alphaSpaceRegex = regexp.MustCompile(`^[\p{L}\s-]+$`)

func init() {
	validate = validator.New()
	// validate.RegisterValidation("e164", validateE164)
	validate.RegisterValidation("adult_birthday", validateAdultBirthday)
	validate.RegisterValidation("complex_password", validateComplexPassword)
	validate.RegisterValidation("valid_phone", validatePhone) 
	validate.RegisterValidation("alpha_space", validateAlphaSpace)

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("form"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

func ValidateStruct(data interface{}) url.Values {
	err := validate.Struct(data)
	if err != nil {
		return formatValidationErrors(err)
	}
	return nil
}

func formatValidationErrors(err error) url.Values {
	errorsMap := url.Values{}
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, fieldErr := range validationErrs {
			fieldName := fieldErr.Field()
			errorsMap.Add(fieldName, getErrorMessage(fieldErr))
		}
	} else {
		errorsMap.Add("general", "Ошибка валидации: "+err.Error())
	}
	return errorsMap
}

func getErrorMessage(err validator.FieldError) string {
	fieldName := err.Field() 
	switch err.Tag() {
	case "required":
		return "Это поле обязательно для заполнения."
	case "email":
		return "Введите корректный адрес электронной почты."
	case "min":
		return fmt.Sprintf("Минимальная длина этого поля: %s символов.", err.Param())
	case "eqfield":
		return fmt.Sprintf("Значение должно совпадать с полем %s.", err.Param())
	case "oneof":
		return fmt.Sprintf("Выберите одно из допустимых значений: %s.", err.Param())
	case "datetime":
		return fmt.Sprintf("Введите дату в формате %s.", err.Param())
	case "adult_birthday":
		return "Пользователь должен быть старше 18 лет."
	case "complex_password":
		return "Пароль должен содержать буквы, цифры и символы."
		case "valid_phone": 
		return "Введите корректный номер телефона (например, +7XXXXXXXXXX)."
	case "alpha_space":
		return "Поле может содержать только буквы, пробелы и дефисы."
	default:
		return fmt.Sprintf("Некорректное значение для поля %s (тег: %s).", fieldName, err.Tag())
	}
}

func validateAlphaSpace(fl validator.FieldLevel) bool {
	return alphaSpaceRegex.MatchString(fl.Field().String())
}

func ValidateAlphaSpace(value string) bool {
	return alphaSpaceRegex.MatchString(value)
}

func validateAdultBirthday(fl validator.FieldLevel) bool {
	birthday := fl.Field().String()
	if birthday == "" {
		return true 
	}
	return auth.IsAdult(birthday)
}

func validateComplexPassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	if password == "" {
		return true 
	}
	return auth.IsPasswordComplex(password)
}

func validatePhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	if phone == "" { return false }
	return auth.ValidatePhone(phone)
}