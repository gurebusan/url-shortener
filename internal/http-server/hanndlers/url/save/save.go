package save

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	resp "url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/sl"
	"url-shortener/internal/lib/random"
	"url-shortener/internal/storage"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
)

type Request struct {
	URL   string `json:"url" validate:"required,url"`
	Alias string `json:"alias,omitempty"`
}

type Response struct {
	resp.Response
	Alias string `json:"alias,omitempty"`
}

// TODO: move to config if needed
const aliasLength = 6

//go:generate go run github.com/vektra/mockery/v2@v2.52.3 --name=URLSaver
type URLSaver interface {
	SaveURL(urlToSave string, alias string) (int64, error)
	AliasChecker
}
type AliasChecker interface {
	CheckAlias(alias string) (bool, error) // Метод для проверки алиаса после генерации
}

func New(log *slog.Logger, urlsaver URLSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.save.New"
		log = slog.With(
			slog.String("op", op),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)

		var req Request

		err := render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			// Такую ошибку встретим, если получили запрос с пустым телом.
			// Обработаем её отдельно
			log.Error("request body is empty")

			render.JSON(w, r, resp.Error("empty request"))

			return
		}
		if err != nil {
			log.Error("failed to decode request body", sl.Err(err))

			render.JSON(w, r, resp.Error("failed to decode request"))
			return
		}
		log.Info("request body decoded", slog.Any("request", req))
		if err := validator.New().Struct(req); err != nil {
			validateErr := err.(validator.ValidationErrors)
			log.Error("invalid request", sl.Err(err))

			render.JSON(w, r, resp.ValidationError(validateErr))
			return
		}
		alias := req.Alias
		if alias == "" {
			generatedAlias, err := GenerateAlias(urlsaver)
			if err != nil {
				log.Error("failed to generate alias", sl.Err(err))
				render.JSON(w, r, resp.Error("failed to generate unique alias, try again"))
				return
			}
			alias = generatedAlias
		}
		id, err := urlsaver.SaveURL(req.URL, alias)
		if errors.Is(err, storage.ErrURLExist) {
			log.Info("url already exist", slog.String("url", req.URL))

			render.JSON(w, r, resp.Error("url already exist"))
			return
		}
		if err != nil {
			log.Error("failed to add url", sl.Err(err))

			render.JSON(w, r, resp.Error("failed to add url"))
			return
		}
		log.Info("url added", slog.Int64("id", id))

		ResponseOK(w, r, alias)
	}
}

func ResponseOK(w http.ResponseWriter, r *http.Request, alias string) {
	render.JSON(w, r, Response{
		Response: resp.OK(),
		Alias:    alias,
	})
}

func GenerateAlias(aliasChecker URLSaver) (string, error) {
	const maxAttempts = 5

	for i := 0; i < maxAttempts; i++ {
		alias := random.NewRandomString(aliasLength) // Генерируем случайный alias
		check, err := aliasChecker.CheckAlias(alias)

		if err != nil {
			return "", fmt.Errorf("failed to check alias: %w", err)
		} else if !check {
			return alias, nil // Нашли уникальный alias, возвращаем его
		}
	}

	// Если за maxAttempts не нашли уникальный alias, возвращаем ошибку
	return "", errors.New("failed to generate unique alias after max attempts")
}
