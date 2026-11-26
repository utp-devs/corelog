package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// CodeFailed is the machine code of DTO validation failures.
const CodeFailed = "validation.failed"

var (
	once     sync.Once
	instance *validator.Validate
)

func Get() *validator.Validate {
	once.Do(func() {
		instance = validator.New(validator.WithRequiredStructEnabled())
	})
	return instance
}

func Struct(s any) error {
	return Get().Struct(s)
}

// Failed turns an error returned by Struct for value s into a validation error
// with code validation.failed. The message and cause are kept as before; the
// failing fields are exposed as Details, named by their JSON key with a Spanish
// message.
func Failed(s any, err error) *apperror.Error {
	appErr := apperror.WrapWithCode(apperror.KindValidation, CodeFailed, "invalid request body", err)
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return appErr
	}
	root := reflect.TypeOf(s)
	details := make([]apperror.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		details = append(details, apperror.FieldError{
			Field:   jsonPath(root, fe.StructNamespace()),
			Message: spanishMessage(fe),
		})
	}
	return appErr.WithDetails(details...)
}

// jsonPath maps a validator namespace such as "createRequest.Address.Street" to
// the JSON names of the fields along the struct path, e.g. "address.street".
func jsonPath(root reflect.Type, namespace string) string {
	parts := strings.Split(namespace, ".")
	if len(parts) > 0 {
		parts = parts[1:] // drop the root struct name
	}
	t := root
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		for t != nil && t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		name := part
		if t != nil && t.Kind() == reflect.Struct {
			if f, ok := t.FieldByName(part); ok {
				name = jsonName(f)
				t = f.Type
			} else {
				t = nil
			}
		} else {
			t = nil
		}
		names = append(names, name)
	}
	return strings.Join(names, ".")
}

func jsonName(f reflect.StructField) string {
	tag := strings.Split(f.Tag.Get("json"), ",")[0]
	if tag == "" || tag == "-" {
		return f.Name
	}
	return tag
}

func spanishMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "es obligatorio"
	case "email":
		return "debe ser un correo válido"
	case "min":
		return fmt.Sprintf("debe tener al menos %s caracteres", fe.Param())
	case "max":
		return fmt.Sprintf("debe tener como máximo %s caracteres", fe.Param())
	case "oneof":
		return fmt.Sprintf("debe ser uno de: %s", fe.Param())
	case "uuid", "uuid4":
		return "debe ser un identificador válido"
	default:
		return "no es válido"
	}
}
