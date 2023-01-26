package app

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue/config"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue/service"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// App is the main application object, containing all services and endpoints
type App struct {
	log    zerolog.Logger
	appCtx *book_catalogue.Context
	cfg    *config.Config
	cancel context.CancelFunc
}

func (a App) Run() error {
	a.log.Info().Str("version", "1.0").Msg("starting SVC")

	// TODO: study context
	_, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// TODO: study package
	var eg errgroup.Group

	// TODO: how to close kafka readers/writers properly on shutdown (graceful shutdown)?
	eg.Go(func() error {
		for {
		}
	})

	// TODO: study mutexes
	return eg.Wait()
}

// NewApp creates a new application object based on passed configuration
func NewApp(cfg *config.Config) (*App, error) {
	appCtx := &book_catalogue.Context{
		Logger: svclog.NewLogger(),
	}

	kafkaReqCh := make(chan service.KafkaMsg, 100)
	kafkaRespCh := make(chan service.KafkaMsg, 100)

	enqHnd := service.NewEnquiryHandler(cfg, appCtx, kafkaReqCh, kafkaRespCh)
	enqHnd.Run()

	kafkaEnquiryProducer := service.NewKafkaEnquiryProducer(cfg, appCtx, kafkaRespCh)
	kafkaEnquiryProducer.Run()
	kafkaEnquiryConsumer := service.NewKafkaEnquiryConsumer(cfg, appCtx, kafkaReqCh)
	kafkaEnquiryConsumer.Run()

	return &App{
		appCtx: appCtx,
		cfg:    cfg,
		log:    svclog.Service(appCtx.Logger, "book-catalogue"),
	}, nil
}
