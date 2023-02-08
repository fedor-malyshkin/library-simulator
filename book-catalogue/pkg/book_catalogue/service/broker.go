package service

import (
	"context"
	"errors"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue"
	"github.com/fedor-malyshkin/library-simulator/book-catalogue/pkg/book_catalogue/config"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
	"time"
)

type KafkaMsg struct {
	Key   string
	Value string
}
type KafkaEnquiryConsumer struct {
	appCtx *book_catalogue.AppContext
	log    zerolog.Logger
	reqCh  chan<- KafkaMsg
	reader *kafka.Reader
}

type KafkaEnquiryProducer struct {
	appCtx *book_catalogue.AppContext
	log    zerolog.Logger
	respCh <-chan KafkaMsg
	writer *kafka.Writer
}

func NewKafkaEnquiryConsumer(cfg *config.Config,
	appCtx *book_catalogue.AppContext,
	ch chan KafkaMsg) *KafkaEnquiryConsumer {

	// make a new reader that consumes from topic
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Kafka.Brokers,
		GroupID:  "book-catalogue",
		Topic:    cfg.Kafka.RequestTopic,
		MinBytes: 1, // https://stackoverflow.com/questions/64656638/using-kafka-go-why-am-i-seeing-what-appears-to-be-batching-reads-writes-is-the
		MaxBytes: 5 * 1024,
		Dialer: &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			ClientID:  "popa",
		}})

	return &KafkaEnquiryConsumer{
		appCtx: appCtx,
		log:    svclog.Service(appCtx.Logger, "kafka-consumer"),
		reader: r,
		reqCh:  ch,
	}
}

func NewKafkaEnquiryProducer(cfg *config.Config,
	appCtx *book_catalogue.AppContext,
	ch chan KafkaMsg) *KafkaEnquiryProducer {

	w := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Brokers...),
		Topic:                  cfg.Kafka.ResponseTopic,
		Balancer:               &kafka.Hash{},
		AllowAutoTopicCreation: true,
		BatchTimeout:           2 * time.Millisecond, // https://github.com/segmentio/kafka-go/issues/326
	}

	return &KafkaEnquiryProducer{
		appCtx: appCtx,
		log:    svclog.Service(appCtx.Logger, "kafka-producer"),
		writer: w,
		respCh: ch,
	}
}

func (c KafkaEnquiryConsumer) MainLoop() error {
	// TODO: what to do if we have an error during reading from Kafka? - Retry?
	defer c.closeConsumer()
	for {
		select {
		case <-c.appCtx.Ctx.Done():
			c.log.Info().Err(c.appCtx.Ctx.Err()).Msg("stop kafka reading loop")
			return c.appCtx.Ctx.Err()
		default:
			_ = c.tryToReadKafka()
		}

	}
}

func (c KafkaEnquiryConsumer) tryToReadKafka() error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	m, err := c.reader.FetchMessage(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil
		} else {
			c.log.Error().Err(err).Msg("kafka reading error")
			c.appCtx.Errors = multierror.Append(c.appCtx.Errors, err)
			c.appCtx.CtxCancelFn()
			return err
		}
	}
	c.log.Debug().Msgf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
	c.reqCh <- KafkaMsg{
		Key:   string(m.Key),
		Value: string(m.Value),
	}
	if err := c.reader.CommitMessages(ctx, m); err != nil {
		c.log.Error().Err(err).Msg("kafka committing error")
		c.appCtx.Errors = multierror.Append(c.appCtx.Errors, err)
		c.appCtx.CtxCancelFn()
		return err
	}
	return nil
}

func (c KafkaEnquiryConsumer) closeConsumer() {
	c.log.Info().Msg("closing the reader")
	if err := c.reader.Close(); err != nil {
		c.log.Err(err).Msg("failed to close reader")
	}
}

func (p KafkaEnquiryProducer) MainLoop() error {
	// TODO: what to do if we have an error during writing into Kafka? - Retry?
	defer p.closeProducer()
	for {
		select {
		case <-p.appCtx.Ctx.Done():
			p.log.Info().Err(p.appCtx.Ctx.Err()).Msg("stop kafka writing loop")
			return p.appCtx.Ctx.Err()
		case msg := <-p.respCh:
			err := p.writeToKafka(msg)
			if err != nil {
				p.log.Error().Err(err).Msg("kafka writing error")
				p.appCtx.Errors = multierror.Append(p.appCtx.Errors, err)
				p.appCtx.CtxCancelFn()
				return err
			}
		}
	}
}

func (p KafkaEnquiryProducer) writeToKafka(msg KafkaMsg) error {
	//------
	start := time.Now()
	//------
	err := p.writer.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(msg.Key),
			Value: []byte(msg.Value),
		})
	// ----------
	elapsed := time.Now().Sub(start)
	p.log.Debug().Dur("writing into kafka duration (ms)", elapsed).Msg("end writing")
	//-----------
	return err
}

func (p KafkaEnquiryProducer) closeProducer() {
	p.log.Info().Msg("closing the writer")
	if err := p.writer.Close(); err != nil {
		p.log.Err(err).Msg("failed to close writer")
	}
}
