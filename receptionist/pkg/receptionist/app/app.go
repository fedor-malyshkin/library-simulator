package app

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/config"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/rest"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/service"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// App is the main application object, containing all services and endpoints
type App struct {
	log    zerolog.Logger
	appCtx *receptionist.Context
	cfg    *config.Config
	routes *rest.MainEndpoint
	cancel context.CancelFunc
}

func (a App) Run() error {
	a.log.Info().Str("version", "1.0").Msg("starting SVC")

	// TODO: study context
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// TODO: study package
	var eg errgroup.Group

	// TODO: how to close kafka readers/writers properly on shutdown (graceful shutdown)?
	eg.Go(func() error {
		return a.routes.Run(ctx)
	})

	// TODO: study mutexes
	return eg.Wait()
}

// NewApp creates a new application object based on passed configuration
func NewApp(cfg *config.Config) (*App, error) {
	appCtx := &receptionist.Context{
		Logger: svclog.NewLogger(),
	}

	kafkaRespCh := make(chan service.KafkaMsg, 100)
	kafkaCancelReqCh := make(chan service.EnquiryID, 100)
	kafkaReqCh := make(chan service.KafkaMsg, 100)

	enquiryHandler := service.NewEnquiryHandler(cfg, appCtx, kafkaReqCh, kafkaCancelReqCh, kafkaRespCh)
	enquiryHandler.Run()
	kafkaEnquiryProducer := service.NewKafkaEnquiryProducer(cfg, appCtx, kafkaReqCh)
	kafkaEnquiryProducer.Run()
	kafkaEnquiryConsumer := service.NewKafkaEnquiryConsumer(cfg, appCtx, kafkaRespCh)
	kafkaEnquiryConsumer.Run()

	ep, err := rest.NewMainEndpoint(cfg, appCtx, enquiryHandler)
	if err != nil {
		return nil, err
	}

	return &App{
		appCtx: appCtx,
		cfg:    cfg,
		log:    svclog.Service(appCtx.Logger, "receptionist"),
		routes: ep,
	}, nil
}
