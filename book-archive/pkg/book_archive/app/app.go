package app

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/config"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/rest"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/service"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// App is the main application object, containing all services and endpoints
type App struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	log       zerolog.Logger
	appCtx    *book_archive.Context
	cfg       *config.Config
	routes    *rest.MainEndpoint
	cancel    context.CancelFunc
}

func (a App) Run() error {
	a.log.Info().Str("version", "1.0").Msg("starting SVC")

	// TODO: study package
	var eg errgroup.Group

	// TODO: how to close kafka readers/writers properly on shutdown (graceful shutdown)?
	eg.Go(func() error {
		return a.routes.Run(a.ctx)
	})

	// TODO: study mutexes
	return eg.Wait()
}

// NewApp creates a new application object based on passed configuration
func NewApp(cfg *config.Config) (*App, error) {
	appCtx := &book_archive.Context{
		Logger: svclog.NewLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	dbHnd := service.NewDBHandler(cfg, appCtx)
	dbHnd.Run(ctx)
	enqHnd := service.NewEnquiryHandler(cfg, appCtx, dbHnd)

	ep, err := rest.NewMainEndpoint(cfg, appCtx, enqHnd)
	if err != nil {
		return nil, err
	}

	return &App{
		ctx:       ctx,
		ctxCancel: cancel,
		appCtx:    appCtx,
		cfg:       cfg,
		routes:    ep,
		log:       svclog.Service(appCtx.Logger, "book-archive"),
	}, nil
}
