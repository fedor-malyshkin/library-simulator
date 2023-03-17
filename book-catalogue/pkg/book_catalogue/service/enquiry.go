package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue/config"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"strings"
	"time"
)

type EnquiryHandler struct {
	appCtx       *book_catalogue.AppContext
	cfg          *config.Config
	log          zerolog.Logger
	rabbitReqCh  <-chan RabbitMsg
	rabbitRespCh chan<- RabbitMsg
}

func NewEnquiryHandler(cfg *config.Config,
	appCtx *book_catalogue.AppContext,
	rabbitReqCh <-chan RabbitMsg,
	rabbitRespCh chan<- RabbitMsg) *EnquiryHandler {

	return &EnquiryHandler{
		appCtx:       appCtx,
		cfg:          cfg,
		log:          svclog.Service(appCtx.Logger, "enquiry-handler"),
		rabbitRespCh: rabbitRespCh,
		rabbitReqCh:  rabbitReqCh,
	}
}

func (h EnquiryHandler) MainLoop() error {
	for {
		select {
		case <-h.appCtx.Ctx.Done():
			h.log.Info().Err(h.appCtx.Ctx.Err()).Msg("stop enquiry processing loop")
			return h.appCtx.Ctx.Err()
		case enq := <-h.rabbitReqCh:
			var resp string
			if len(enq.Value) < 20 {
				var err error
				ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
				resp, err = h.askBookArchive(ctx, enq.Value)
				cancel()
				if err != nil {
					h.log.Warn().Err(err).Msg("asking book-archive")
					continue
				}

			} else {
				resp = strings.Replace(enq.Value, "a", "z", -1)
			}
			// h.log.Info().Msgf("<%s> -> <%s>", enq.Value, resp)
			h.rabbitRespCh <- RabbitMsg{
				OrgMsg: enq.OrgMsg,
				Value:  resp,
			}
		}
	}
}

func (h EnquiryHandler) askBookArchive(ctx context.Context, q string) (string, error) {
	url := fmt.Sprintf("http://%s:%d/archive?q=%s", h.cfg.BookArchive.IP, h.cfg.BookArchive.Port, q)
	var result string
	var resultErr error
	respCh := make(chan struct{})
	go func() {
		result, resultErr = h.callREST(url)
		close(respCh)
	}()

	select {
	case <-ctx.Done():
		return "", errors.New("GitHub Timeout")
	case <-respCh:
		return result, resultErr
	}

}

func (h EnquiryHandler) callREST(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
