package rest

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/config"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/service"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// MainEndpoint is main endpoint
type MainEndpoint struct {
	log     zerolog.Logger
	httpSrv *http.Server
	handler *service.EnquiryHandler
}

func NewMainEndpoint(cfg *config.Config,
	appCtx *receptionist.Context,
	enquiryHandler *service.EnquiryHandler) (*MainEndpoint, error) {
	srv := http.Server{
		Addr:    net.JoinHostPort(cfg.Http.IP, strconv.Itoa(int(cfg.Http.Port))),
		Handler: setupRoutes(cfg, enquiryHandler),
	}
	return &MainEndpoint{
		log:     svclog.Service(appCtx.Logger, "route-endpoint"),
		httpSrv: &srv,
		handler: enquiryHandler,
	}, nil
}

func (e MainEndpoint) Run(ctx context.Context) error {
	e.log.Info().Msgf("Starting SVC endpoint on %s", e.httpSrv.Addr)
	e.httpSrv.BaseContext = func(listener net.Listener) context.Context { return ctx }
	return e.httpSrv.ListenAndServe()
}

func setupRoutes(cfg *config.Config, handler *service.EnquiryHandler) *gin.Engine {
	r := gin.New()
	r.POST("/enquiry", NewRestEnquiryHandler(handler))
	return r
}

func NewRestEnquiryHandler(hnd *service.EnquiryHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		cd, cont := hnd.ProcessEnquiry(ctx, c.Request)
		c.String(cd, "%s", cont)
		c.Header("Content-Type", "application/json")
	}
}
