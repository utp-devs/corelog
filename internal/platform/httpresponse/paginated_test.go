package httpresponse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaginatedWritesDataAndMeta(t *testing.T) {
	recorder := httptest.NewRecorder()
	Paginated(recorder, http.StatusOK, []string{"a", "b"}, PageMeta{Page: 2, PageSize: 20, Total: 41})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		Data []string `json:"data"`
		Meta PageMeta `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 2 || body.Meta.Total != 41 || body.Meta.Page != 2 || body.Meta.PageSize != 20 {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}
