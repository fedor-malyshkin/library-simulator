package app

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/config"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/rest"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/service"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
)

// App is the main application object, containing all services and endpoints
type App struct {
	appCtx *book_archive.AppContext
	log    zerolog.Logger
	cfg    *config.Config
	routes *rest.MainEndpoint
}

func (a App) Run() error {
	a.log.Info().Str("version", "1.0").Msg("starting SVC")

	a.appCtx.ErrGroup.Go(a.routes.ListenAndServe)
	a.appCtx.ErrGroup.Go(a.routes.ShutdownListenerLoop)

	err := a.appCtx.ErrGroup.Wait()
	return multierror.Append(a.appCtx.Errors, err).ErrorOrNil()
}

// NewApp creates a new application object based on passed configuration
func NewApp(cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())
	appCtx := book_archive.NewAppContext(svclog.NewLogger(), ctx, cancel)

	dbHnd := service.NewDBHandler(cfg, appCtx)
	appCtx.ErrGroup.Go(dbHnd.MainLoop)
	enqHnd := service.NewEnquiryHandler(cfg, appCtx, dbHnd)

	ep, err := rest.NewMainEndpoint(cfg, appCtx, enqHnd)
	if err != nil {
		return nil, err
	}

	return &App{
		appCtx: appCtx,
		cfg:    cfg,
		routes: ep,
		log:    svclog.Service(appCtx.Logger, "book-archive"),
	}, nil
}
