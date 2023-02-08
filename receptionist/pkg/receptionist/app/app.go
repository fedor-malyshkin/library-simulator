package app

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/config"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/rest"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/service"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
)

// App is the main application object, containing all services and endpoints
type App struct {
	log    zerolog.Logger
	appCtx *receptionist.AppContext
	cfg    *config.Config
	routes *rest.MainEndpoint
}

func (a App) StartApp() error {
	a.log.Info().Str("version", "1.0").Msg("starting SVC")

	a.appCtx.ErrGroup.Go(a.routes.ListenAndServe)
	a.appCtx.ErrGroup.Go(a.routes.ShutdownListenerLoop)

	err := a.appCtx.ErrGroup.Wait()
	return multierror.Append(a.appCtx.Errors, err).ErrorOrNil()
}

// NewApp creates a new application object based on passed configuration
func NewApp(cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	appCtx := receptionist.NewAppContext(svclog.NewLogger(), ctx, cancel)

	kafkaRespCh := make(chan service.KafkaMsg, 100)
	kafkaCancelReqCh := make(chan service.EnquiryID, 100)
	kafkaReqCh := make(chan service.KafkaMsg, 100)

	enquiryHandler := service.NewEnquiryHandler(cfg, appCtx, kafkaReqCh, kafkaCancelReqCh, kafkaRespCh)
	appCtx.ErrGroup.Go(enquiryHandler.MainLoop)
	kafkaEnquiryProducer := service.NewKafkaEnquiryProducer(cfg, appCtx, kafkaReqCh)
	appCtx.ErrGroup.Go(kafkaEnquiryProducer.MainLoop)
	kafkaEnquiryConsumer := service.NewKafkaEnquiryConsumer(cfg, appCtx, kafkaRespCh)
	appCtx.ErrGroup.Go(kafkaEnquiryConsumer.MainLoop)

	ep, err := rest.NewMainEndpoint(cfg, appCtx, enquiryHandler)
	if err != nil {
		return nil, err
	}

	return &App{
		log:    svclog.Service(appCtx.Logger, "receptionist"),
		appCtx: appCtx,
		cfg:    cfg,
		routes: ep,
	}, nil
}
