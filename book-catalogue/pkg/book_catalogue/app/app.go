package app

import (
	"context"
	"errors"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue/config"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue/service"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
)

// App is the main application object, containing all services and endpoints
type App struct {
	log    zerolog.Logger
	appCtx *book_catalogue.AppContext
	cfg    *config.Config
}

func (a App) StartApp() error {
	a.log.Info().Str("version", "1.0").Msg("starting SVC")

	err := a.appCtx.ErrGroup.Wait()
	errs := multierror.Append(a.appCtx.Errors, err)
	return filterOutCancelErr(errs).ErrorOrNil()
}

func filterOutCancelErr(ers *multierror.Error) *multierror.Error {
	var res error
	for _, e := range ers.WrappedErrors() {
		if errors.Is(e, context.Canceled) {
			continue
		}
		res = multierror.Append(res, e)
	}
	return multierror.Append(res, nil)
}

// NewApp creates a new application object based on passed configuration
func NewApp(cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	appCtx := book_catalogue.NewAppContext(svclog.NewLogger(), ctx, cancel)

	kafkaReqCh := make(chan service.KafkaMsg, 100)
	kafkaRespCh := make(chan service.KafkaMsg, 100)

	enqHnd := service.NewEnquiryHandler(cfg, appCtx, kafkaReqCh, kafkaRespCh)
	appCtx.ErrGroup.Go(enqHnd.MainLoop)

	kafkaEnquiryProducer := service.NewKafkaEnquiryProducer(cfg, appCtx, kafkaRespCh)
	appCtx.ErrGroup.Go(kafkaEnquiryProducer.MainLoop)

	kafkaEnquiryConsumer := service.NewKafkaEnquiryConsumer(cfg, appCtx, kafkaReqCh)
	appCtx.ErrGroup.Go(kafkaEnquiryConsumer.MainLoop)

	return &App{
		log:    svclog.Service(appCtx.Logger, "book-catalogue"),
		appCtx: appCtx,
		cfg:    cfg,
	}, nil
}
