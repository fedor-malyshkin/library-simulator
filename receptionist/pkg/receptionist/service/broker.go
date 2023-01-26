package service

import (
	"context"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/config"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"time"
)

type KafkaMsg struct {
	Key   string
	Value string
}
type KafkaEnquiryConsumer struct {
	log    zerolog.Logger
	respCh chan<- KafkaMsg
	reader *kafka.Reader
}

type KafkaEnquiryProducer struct {
	log    zerolog.Logger
	reqCh  <-chan KafkaMsg
	writer *kafka.Writer
}

func NewKafkaEnquiryConsumer(cfg *config.Config,
	appCtx *receptionist.Context,
	ch chan KafkaMsg) *KafkaEnquiryConsumer {

	// make a new reader that consumes from topic
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Kafka.Brokers,
		GroupID:  uuid.New().String(),
		Topic:    cfg.Kafka.ResponseTopic,
		MinBytes: 1, // https://stackoverflow.com/questions/64656638/using-kafka-go-why-am-i-seeing-what-appears-to-be-batching-reads-writes-is-the
		MaxBytes: 5 * 1024,
	})

	return &KafkaEnquiryConsumer{
		log:    svclog.Service(appCtx.Logger, "kafka-consumer"),
		reader: r,
		respCh: ch,
	}
}

func NewKafkaEnquiryProducer(cfg *config.Config,
	appCtx *receptionist.Context,
	ch chan KafkaMsg) *KafkaEnquiryProducer {

	w := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Brokers...),
		Topic:                  cfg.Kafka.RequestTopic,
		Balancer:               &kafka.Hash{},
		AllowAutoTopicCreation: true,
		BatchTimeout:           2 * time.Millisecond, // https://github.com/segmentio/kafka-go/issues/326
	}

	return &KafkaEnquiryProducer{
		log:    svclog.Service(appCtx.Logger, "kafka-producer"),
		writer: w,
		reqCh:  ch,
	}
}

func (c KafkaEnquiryConsumer) Run() {
	// TODO: what to do if we have an error during reading from Kafka? - Retry?
	go func() {
		for {
			ctx := context.Background()
			m, err := c.reader.FetchMessage(ctx)
			if err != nil {
				c.log.Error().Err(err).Msg("kafka reading error")
				break
			}
			c.log.Debug().Msgf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
			c.respCh <- KafkaMsg{
				Key:   string(m.Key),
				Value: string(m.Value),
			}
			if err := c.reader.CommitMessages(ctx, m); err != nil {
				c.log.Error().Err(err).Msg("failed to commit message")
				break
			}
		}

		if err := c.reader.Close(); err != nil {
			log.Fatal().Err(err).Msg("failed to close reader")
		}
	}()
}

func (p KafkaEnquiryProducer) Run() {
	// TODO: what to do if we have an error during writing into Kafka? - Retry?
	go func() {
		for {
			msg := <-p.reqCh
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
			if err != nil {
				p.log.Error().Err(err).Msg("kafka writing error")
				break
			}
		}

		if err := p.writer.Close(); err != nil {
			log.Fatal().Err(err).Msg("failed to close writer")
		}
	}()
}
