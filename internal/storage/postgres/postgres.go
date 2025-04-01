package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"
	"url-shortener/internal/config"
	"url-shortener/internal/storage"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Storage struct {
	db *pgxpool.Pool
}

func New(cfg config.StorageConnection) (*Storage, error) {
	const op = "storage.postgres.New"

	//Создаём конфиг пула
	poolConfig, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSLMode,
	))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	//Создаем пул
	pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	//Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	//Создаём таблицу, если она не существует
	query := `
	CREATE TABLE IF NOT EXISTS urls (
        id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
        url TEXT NOT NULL,
        alias TEXT NOT NULL UNIQUE
    );
	`
	_, err = pool.Exec(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &Storage{db: pool}, nil
}

func (s *Storage) SaveURL(urlToSave string, alias string) (int64, error) {
	const op = "storage.postgres.SaveURL"

	//Выполняем запрос
	query := "INSERT INTO urls(url, alias) VALUES($1, $2) RETURNING id"
	var id int64
	err := s.db.QueryRow(context.Background(), query, urlToSave, alias).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrURLExist)
		}
		return 0, fmt.Errorf("%s: failed to execute query: %w", op, err)
	}
	//Возращаем id полученной записи
	return id, nil
}

func (s *Storage) GetURL(alias string) (string, error) {
	const op = "storage.postgres.GetURL"
	//Выполняем запрос
	query := "SELECT url FROM urls WHERE alias = $1"
	var resURL string
	err := s.db.QueryRow(context.Background(), query, alias).Scan(&resURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrURLNotFound)
		}
		return "", fmt.Errorf("%s: failed to execute query: %w", op, err)
	}
	return resURL, nil
}

func (s *Storage) DeleteURL(alias string) (string, error) {
	const op = "storage.postgres.DeleteURL"

	var resURL string
	selectQuery := "SELECT url FROM urls WHERE alias = $1"
	err := s.db.QueryRow(context.Background(), selectQuery, alias).Scan(&resURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrURLNotFound)
		}
		return "", fmt.Errorf("%s: failed to fetch URL: %w", op, err)
	}

	deleteQuery := "DELETE FROM urls WHERE alias = $1"
	_, err = s.db.Exec(context.Background(), deleteQuery, alias)
	if err != nil {
		return "", fmt.Errorf("%s: failed to delete url: %w", op, err)
	}

	return resURL, nil
}
