package rest

import (
	"context"
	"errors"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/config"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/service"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// MainEndpoint is main endpoint
type MainEndpoint struct {
	appCtx  *book_archive.AppContext
	log     zerolog.Logger
	httpSrv *http.Server
	handler *service.EnquiryHandler
}

func NewMainEndpoint(cfg *config.Config,
	appCtx *book_archive.AppContext,
	enquiryHandler *service.EnquiryHandler) (*MainEndpoint, error) {
	srv := http.Server{
		Addr:    net.JoinHostPort(cfg.Http.IP, strconv.Itoa(int(cfg.Http.Port))),
		Handler: setupRoutes(cfg, enquiryHandler),
	}
	return &MainEndpoint{
		appCtx:  appCtx,
		log:     svclog.Service(appCtx.Logger, "route-endpoint"),
		httpSrv: &srv,
		handler: enquiryHandler,
	}, nil
}

func (e MainEndpoint) ListenAndServe() error {
	e.log.Info().Msgf("Starting SVC endpoint on %s", e.httpSrv.Addr)
	e.httpSrv.BaseContext = func(listener net.Listener) context.Context { return e.appCtx.Ctx }
	err := e.httpSrv.ListenAndServe()
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	return nil
}

func (e MainEndpoint) ShutdownListenerLoop() error {
	<-e.appCtx.Ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return e.httpSrv.Shutdown(ctx)
}

func setupRoutes(cfg *config.Config, handler *service.EnquiryHandler) *gin.Engine {
	r := gin.New()
	r.GET("/archive", NewRestEnquiryHandler(handler))
	return r
}

func NewRestEnquiryHandler(hnd *service.EnquiryHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		cd, cont := hnd.ProcessEnquiry(c.Query("q"))
		c.String(cd, "%s", cont)
		c.Header("Content-Type", "application/json")

	}
}
