package service

import (
	"context"
	"errors"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/config"
	"github.com/hashicorp/go-multierror"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"time"
)

type RabbitMsg struct {
	Key   string
	Value string
}
type RabbitEnquiryProcessor struct {
	appCtx *receptionist.AppContext
	log    zerolog.Logger

	respCh chan<- RabbitMsg
	reqCh  <-chan RabbitMsg

	rbConn  *amqp.Connection
	rbCh    *amqp.Channel
	rbQueue amqp.Queue
	rbMsgCh <-chan amqp.Delivery
}

func NewRabbitEnquiryProcessor(cfg *config.Config,
	appCtx *receptionist.AppContext,
	reqCh <-chan RabbitMsg,
	respCh chan<- RabbitMsg) *RabbitEnquiryProcessor {

	log := svclog.Service(appCtx.Logger, "rabbit-rpc-server")

	rbConn, err := amqp.Dial("amqp://rmuser:rmpassword@localhost:5672/")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to broker")
	}
	rbCh, err := rbConn.Channel()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot open broker channel")
	}
	rbQueue, err := rbCh.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot declare the queue")
	}
	rbMsgCh, err := rbCh.Consume(
		rbQueue.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start message consumption")
	}

	return &RabbitEnquiryProcessor{
		appCtx:  appCtx,
		log:     log,
		respCh:  respCh,
		reqCh:   reqCh,
		rbConn:  rbConn,
		rbCh:    rbCh,
		rbQueue: rbQueue,
		rbMsgCh: rbMsgCh,
	}
}

func (s RabbitEnquiryProcessor) MainRequestLoop() error {
	// TODO: what to do if we have an error during reading from Kafka? - Retry?
	defer s.closeRPCServer()
	for {
		select {
		case <-s.appCtx.Ctx.Done():
			s.log.Info().Err(s.appCtx.Ctx.Err()).Msg("stop rabbit request loop")
			return s.appCtx.Ctx.Err()
		case msg := <-s.reqCh:
			err := s.requestRpc(msg)
			if err != nil {
				s.log.Error().Err(err).Msg("rabbit writing error")
				s.appCtx.Errors = multierror.Append(s.appCtx.Errors, err)
				s.appCtx.CtxCancelFn()
				return err
			}
		}

	}
}

func (s RabbitEnquiryProcessor) requestRpc(msg RabbitMsg) error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := s.rbCh.PublishWithContext(ctx,
		"",          // exchange
		"rpc_queue", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: msg.Key,
			ReplyTo:       s.rbQueue.Name,
			Body:          []byte(msg.Value),
		})

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil
		} else {
			s.log.Error().Err(err).Msg("rabbit reading error")
			s.appCtx.Errors = multierror.Append(s.appCtx.Errors, err)
			s.appCtx.CtxCancelFn()
			return err
		}
	}
	return nil
}

func (s RabbitEnquiryProcessor) MainResponseLoop() error {
	for {
		select {
		case <-s.appCtx.Ctx.Done():
			s.log.Info().Err(s.appCtx.Ctx.Err()).Msg("stop rabbit response loop")
			return s.appCtx.Ctx.Err()
		case msg, ok := <-s.rbMsgCh:
			if !ok {
				s.log.Info().Msg("rabbit response chanel is closed")
				s.appCtx.CtxCancelFn()
				return nil
			}
			s.respCh <- RabbitMsg{msg.CorrelationId, string(msg.Body)}
		}
	}
}

func (s RabbitEnquiryProcessor) closeRPCServer() {
	s.log.Info().Msg("Closing the reader")
	s.rbCh.Close()
	s.rbConn.Close()
}
