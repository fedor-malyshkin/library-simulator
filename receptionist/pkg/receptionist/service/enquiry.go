package service

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/config"
	"github.com/google/uuid"

	"github.com/rs/zerolog"
	"io"
	"net/http"
	"strings"
)

type EnquiryID string

func (e EnquiryID) Str() string {
	return string(e)
}

type EnquiryReq struct {
	id      EnquiryID
	request string
	respCh  chan<- EnquiryResp
}

type EnquiryResp struct {
	id       EnquiryID
	response string
}

type EnquiryHandler struct {
	appCtx           *receptionist.AppContext
	log              zerolog.Logger
	reqMap           map[EnquiryID]chan<- EnquiryResp
	enqReqCh         chan EnquiryReq
	kafkaReqCh       chan<- KafkaMsg
	kafkaCancelReqCh chan EnquiryID
	kafkaRespCh      <-chan KafkaMsg
}

func NewEnquiryHandler(cfg *config.Config,
	appCtx *receptionist.AppContext,
	kafkaReqCh chan KafkaMsg,
	kafkaCancelReqCh chan EnquiryID,
	kafkaRespCh chan KafkaMsg) *EnquiryHandler {
	return &EnquiryHandler{
		appCtx:           appCtx,
		log:              svclog.Service(appCtx.Logger, "enquiry-handler"),
		reqMap:           make(map[EnquiryID]chan<- EnquiryResp, 100),
		enqReqCh:         make(chan EnquiryReq, 100),
		kafkaRespCh:      kafkaRespCh,
		kafkaCancelReqCh: kafkaCancelReqCh,
		kafkaReqCh:       kafkaReqCh,
	}
}

func (h EnquiryHandler) ProcessEnquiry(ctx context.Context, request *http.Request) (int, string) {
	bd := request.Body
	defer bd.Close()
	buf := new(strings.Builder)
	_, err := io.Copy(buf, bd)
	if err != nil {
		h.log.Err(err)
		return http.StatusBadRequest, ""
	}
	h.log.Info().Str("content", buf.String()).Msg("incoming request")

	respCh := make(chan EnquiryResp)
	enqID := EnquiryID(uuid.New().String())
	h.SendEnquiry(enqID, buf.String(), respCh)

	select {
	case <-ctx.Done():
		h.kafkaCancelReqCh <- enqID
		h.log.Warn().Str("EnquiryID", enqID.Str()).Msg("kafka response timeout")
		return http.StatusRequestTimeout, ""
	case resp := <-respCh:
		close(respCh)
		return http.StatusOK, resp.response
	}

}

func (h EnquiryHandler) SendEnquiry(id EnquiryID, content string, ch chan EnquiryResp) {
	h.enqReqCh <- EnquiryReq{
		id:      id,
		request: content,
		respCh:  ch,
	}
}

func (h EnquiryHandler) MainLoop() error {
	for {
		select {
		case <-h.appCtx.Ctx.Done():
			h.log.Info().Err(h.appCtx.Ctx.Err()).Msg("stop request processing loop")
			return h.appCtx.Ctx.Err()
		case enqID := <-h.kafkaCancelReqCh:
			if ch, ok := h.reqMap[enqID]; ok {
				close(ch)
			}
			delete(h.reqMap, enqID)
		case enqReq := <-h.enqReqCh:
			h.reqMap[enqReq.id] = enqReq.respCh
			h.log.Info().Str("EnquiryID", enqReq.id.Str()).Msg("put ID into map")
			h.kafkaReqCh <- KafkaMsg{
				Key:   enqReq.id.Str(),
				Value: enqReq.request,
			}
		case kfkResp := <-h.kafkaRespCh:
			enqID := EnquiryID(kfkResp.Key)
			if ch, ok := h.reqMap[enqID]; ok {
				ch <- EnquiryResp{
					id:       enqID,
					response: kfkResp.Value,
				}
				delete(h.reqMap, enqID)
			} else {
				h.log.Info().Str("EnquiryID", enqID.Str()).Msg("unknown message ID (probably time-outed)")
			}
		}
	}
}
