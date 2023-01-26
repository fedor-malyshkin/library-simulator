package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/config"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	log     zerolog.Logger
	db      *sqlx.DB
	cacheCh chan CacheRec
}

type CacheRec struct {
	Id    string
	Query string
	Data  string
}

const appMigVer = 1

func NewDBHandler(cfg *config.Config,
	appCtx *book_archive.Context) *Handler {

	cnStr := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", cfg.DB.User, cfg.DB.Password,
		cfg.DB.Host, cfg.DB.Database)
	db := createDB(cfg, appCtx, cnStr)
	migrateDB(cnStr, appCtx)

	return &Handler{
		log:     svclog.Service(appCtx.Logger, "db-handler"),
		db:      db,
		cacheCh: make(chan CacheRec, 100),
	}
}

func migrateDB(cnStr string, appCtx *book_archive.Context) {
	m, err := migrate.New("file:./migrations", cnStr)
	if err != nil {
		appCtx.Logger.Fatal().Err(err).Msg("cannot configure DB migrations")
	}
	curDBVer, _, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			appCtx.Logger.Info().Msg("no DB migrations")
		} else {
			appCtx.Logger.Fatal().Err(err).Msg("cannot perform DB migrations")
		}
	}
	if appMigVer <= curDBVer {
		appCtx.Logger.Info().Msgf("no DB migration is needed - current DB version: %d, app version: %d",
			curDBVer, appMigVer)
		return
	}
	if err = m.Up(); err != nil {
		appCtx.Logger.Fatal().Err(err).Msg("cannot run DB migrations")
	}
}

func createDB(cfg *config.Config, appCtx *book_archive.Context, cnStr string) *sqlx.DB {
	db, err := sqlx.Connect("pgx", cnStr)
	if err != nil {
		appCtx.Logger.Fatal().Err(err).Msg("cannot connect to DB")
	}
	return db
}

func (h *Handler) Run(ctx context.Context) {
	// TODO: how to finish this infinite goroutine?
	go func() {
		for {
			select {
			case <-ctx.Done():
				h.log.Info().Msg("end run-loop")
				return
			case q := <-h.cacheCh:
				h.storeToCache(q)
			}
		}
	}()
}

func (h *Handler) storeToCache(cr CacheRec) {
	insertQuery := "INSERT INTO archive (query, Data) VALUES ($1, $2) ON CONFLICT DO NOTHING"
	h.db.MustExec(insertQuery, cr.Query, cr.Data)
}

func (h *Handler) getFromCache(q string) (string, bool) {
	var cr CacheRec
	err := h.db.QueryRowx("SELECT id, query, data FROM archive WHERE query LIKE $1", q).
		StructScan(&cr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false
		}
		log.Fatal().Err(err).Msg("unable to query from DB archive")
	}
	return cr.Data, true
}
