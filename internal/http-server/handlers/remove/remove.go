package remove

import (
	"errors"
	"log/slog"
	"net/http"

	resp "url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/sl"
	"url-shortener/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

//go:generate go run github.com/vektra/mockery/v2@v2.52.3 --name=URLRemover
type URLRemover interface {
	DeleteURL(alias string) (string, error)
}

func New(log *slog.Logger, urlremover URLRemover) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.remove.New"
		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)
		alias := chi.URLParam(r, "alias")
		if alias == "" {
			log.Info("alias is empty")
			render.JSON(w, r, resp.Error("Invalid requset"))
			return
		}

		resURL, err := urlremover.DeleteURL(alias)
		if err != nil {
			if errors.Is(err, storage.ErrURLNotFound) {
				log.Info("url not exist", "alias", alias)

				render.JSON(w, r, resp.Error("not exist"))
				return
			}
			log.Info("failed to delete url", sl.Err(err))

			render.JSON(w, r, resp.Error("Internal error"))
			return
		}
		log.Info("deleted url", slog.String("url", resURL))

		render.JSON(w, r, resp.OK())
	}
}
