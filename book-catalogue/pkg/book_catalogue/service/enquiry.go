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
)

type EnquiryHandler struct {
	cfg         *config.Config
	log         zerolog.Logger
	kafkaReqCh  <-chan KafkaMsg
	kafkaRespCh chan<- KafkaMsg
}

func NewEnquiryHandler(cfg *config.Config,
	appCtx *book_catalogue.Context,
	kafkaReqCh <-chan KafkaMsg,
	kafkaRespCh chan<- KafkaMsg) *EnquiryHandler {

	return &EnquiryHandler{
		cfg:         cfg,
		log:         svclog.Service(appCtx.Logger, "enquiry-handler"),
		kafkaRespCh: kafkaRespCh,
		kafkaReqCh:  kafkaReqCh,
	}
}

func (h EnquiryHandler) Run() {
	go func() {
		for {
			enq := <-h.kafkaReqCh
			var resp string
			if len(enq.Value) < 20 {
				var err error
				// ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
				ctx, cancel := context.WithCancel(context.Background())
				resp, err = h.askBookArchive(ctx, enq.Value)
				if err != nil {
					h.log.Err(err).Msg("asking book-archive")
					continue
				}
				cancel()
			} else {
				resp = strings.Replace(enq.Value, "a", "z", -1)
			}
			h.log.Info().Msgf("<%s> -> <%s>", enq.Value, resp)
			h.kafkaRespCh <- KafkaMsg{
				Key:   enq.Key,
				Value: resp,
			}
		}
	}()
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
