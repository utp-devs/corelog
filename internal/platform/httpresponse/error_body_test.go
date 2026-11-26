package httpresponse

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestErrorBodyCodeAndDetails(t *testing.T) {
	details := []apperror.FieldError{{Field: "title", Message: "is required"}}

	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantDetails []apperror.FieldError
		wantRawKeys []string
		omitKeys    []string
	}{
		{
			name:        "code and details present",
			err:         apperror.WithCode(apperror.KindValidation, "issue_invalid", "datos invalidos").WithDetails(details...),
			wantStatus:  http.StatusUnprocessableEntity,
			wantCode:    "issue_invalid",
			wantDetails: details,
			wantRawKeys: []string{"code", "details"},
		},
		{
			name:       "code and details omitted when empty",
			err:        apperror.New(apperror.KindNotFound, "no encontrado"),
			wantStatus: http.StatusNotFound,
			omitKeys:   []string{"code", "details"},
		},
		{
			name:        "rate limited with code maps to 429",
			err:         apperror.WithCode(apperror.KindRateLimited, "rate_limited", "demasiadas peticiones"),
			wantStatus:  http.StatusTooManyRequests,
			wantCode:    "rate_limited",
			wantRawKeys: []string{"code"},
			omitKeys:    []string{"details"},
		},
		{
			name:       "plain error has no code or details",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			omitKeys:   []string{"code", "details"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Error(rec, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var raw map[string]map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatalf("decode: %v", err)
			}
			body := raw["error"]
			for _, k := range tt.wantRawKeys {
				if _, ok := body[k]; !ok {
					t.Errorf("key %q missing in %s", k, rec.Body.String())
				}
			}
			for _, k := range tt.omitKeys {
				if _, ok := body[k]; ok {
					t.Errorf("key %q must be omitted, body %s", k, rec.Body.String())
				}
			}

			var typed struct {
				Error struct {
					Code    string                `json:"code"`
					Details []apperror.FieldError `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &typed); err != nil {
				t.Fatalf("decode typed: %v", err)
			}
			if typed.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", typed.Error.Code, tt.wantCode)
			}
			if !reflect.DeepEqual(typed.Error.Details, tt.wantDetails) {
				t.Errorf("details = %+v, want %+v", typed.Error.Details, tt.wantDetails)
			}
			if strings.Contains(rec.Body.String(), `"details":null`) {
				t.Error("details must never be serialized as null")
			}
		})
	}
}
