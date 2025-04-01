package remove_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"url-shortener/internal/http-server/handlers/remove"
	"url-shortener/internal/http-server/handlers/remove/mocks"
	"url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/handlers/slogdiscard"
	"url-shortener/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveHandler(t *testing.T) {
	cases := []struct {
		name      string
		alias     string
		url       string
		respError string
		mockError error
	}{
		{
			name:  "Success",
			alias: "test_alias",
			url:   "https://google.com",
		},
		{
			name:      "URL not found",
			alias:     "not_exist",
			respError: "not exist",
			mockError: storage.ErrURLNotFound,
		},
		{
			name:      "Internal error",
			alias:     "db_fail",
			respError: "Internal error",
			mockError: errors.New("db error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			urlRemoverMock := mocks.NewURLRemover(t)

			urlRemoverMock.On("DeleteURL", tc.alias).
				Return(tc.url, tc.mockError).Once()

			r := chi.NewRouter()
			r.Delete("/{alias}", remove.New(slogdiscard.NewDiscardLogger(), urlRemoverMock))

			req := httptest.NewRequest(http.MethodDelete, "/"+tc.alias, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Проверяем, что всегда возвращается статус 200
			assert.Equal(t, http.StatusOK, w.Code)

			// Декодируем JSON-ответ
			var resp response.Response
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			// Если ожидалась ошибка - проверяем текст ошибки
			if tc.respError != "" {
				assert.Equal(t, response.StatusError, resp.Status)
				assert.Equal(t, tc.respError, resp.Error)
			} else {
				assert.Equal(t, response.StatusOK, resp.Status)
				assert.Empty(t, resp.Error)
			}
		})
	}
}
