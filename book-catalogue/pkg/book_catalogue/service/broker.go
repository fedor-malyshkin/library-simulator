package service

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue/config"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/hashicorp/go-multierror"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"time"
)

type RabbitMsg struct {
	Value  string
	OrgMsg amqp.Delivery
}

type RabbitEnquiryProcessor struct {
	appCtx  *book_catalogue.AppContext
	log     zerolog.Logger
	reqCh   chan<- RabbitMsg
	respCh  <-chan RabbitMsg
	rbConn  *amqp.Connection
	rbCh    *amqp.Channel
	rbQueue amqp.Queue
	rbMsgCh <-chan amqp.Delivery
}

func NewRabbitEnquiryProcessor(cfg *config.Config,
	appCtx *book_catalogue.AppContext,
	reqCh chan<- RabbitMsg,
	respCh <-chan RabbitMsg) *RabbitEnquiryProcessor {

	log := svclog.Service(appCtx.Logger, "rabbit-processor")

	rbConn, err := amqp.Dial("amqp://rmuser:rmpassword@localhost:5672/")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to broker")
	}
	rbCh, err := rbConn.Channel()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot open broker channel")
	}
	rbQueue, err := rbCh.QueueDeclare(
		"rpc_queue", // name
		false,       // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot declare the queue")
	}
	err = rbCh.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot set QoS")
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
		log:     svclog.Service(appCtx.Logger, "rabbit-consumer"),
		reqCh:   reqCh,
		respCh:  respCh,
		rbConn:  rbConn,
		rbCh:    rbCh,
		rbQueue: rbQueue,
		rbMsgCh: rbMsgCh,
	}
}

func (p RabbitEnquiryProcessor) MainRequestLoop() error {
	// TODO: what to do if we have an error during reading from Kafka? - Retry?
	defer p.closeConsumer()
	for {
		select {
		case <-p.appCtx.Ctx.Done():
			p.log.Info().Err(p.appCtx.Ctx.Err()).Msg("stop rabbit reading loop")
			return p.appCtx.Ctx.Err()
		case msg, ok := <-p.rbMsgCh:
			if !ok {
				p.log.Info().Msg("rabbit request chanel is closed")
				p.appCtx.CtxCancelFn()
				return nil
			}
			p.reqCh <- RabbitMsg{OrgMsg: msg,
				Value: string(msg.Body)}
		}
	}
}

func (p RabbitEnquiryProcessor) closeConsumer() {
	p.log.Info().Msg("Closing the reader")
	p.rbCh.Close()
	p.rbConn.Close()
}

func (p RabbitEnquiryProcessor) MainResponseLoop() error {
	// TODO: what to do if we have an error during writing into Kafka? - Retry?
	for {
		select {
		case <-p.appCtx.Ctx.Done():
			p.log.Info().Err(p.appCtx.Ctx.Err()).Msg("stop rabbit writing loop")
			return p.appCtx.Ctx.Err()
		case msg := <-p.respCh:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := p.rbCh.PublishWithContext(ctx,
				"",                 // exchange
				msg.OrgMsg.ReplyTo, // routing key
				false,              // mandatory
				false,              // immediate
				amqp.Publishing{
					ContentType:   "text/plain",
					CorrelationId: msg.OrgMsg.CorrelationId,
					Body:          []byte(msg.Value),
				})
			cancel()
			if err != nil {
				p.log.Error().Err(err).Msg("rabbit writing resp error")
				p.appCtx.Errors = multierror.Append(p.appCtx.Errors, err)
				p.appCtx.CtxCancelFn()
				return err
			}
		}
	}
}
