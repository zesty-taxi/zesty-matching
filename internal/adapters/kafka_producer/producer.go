package kafka_producer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"github.com/zesty-taxi/zesty-matching/internal/domain"
)

type Producer struct {
	writers map[string]*kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	topics := []string{
		domain.TopicRideOfferCreated,
		domain.TopicRideDriverAssigned,
		domain.TopicRideMatchingFailed,
		domain.TopicMatchingDLQ,
	}

	writers := make(map[string]*kafka.Writer, len(topics))
	for _, topic := range topics {
		writers[topic] = &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  3,
			BatchTimeout: 10 * time.Millisecond,
		}
	}

	return &Producer{writers: writers}
}

func (p *Producer) Close() {
	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close producer")
		}
	}
}

func (p *Producer) PublishRaw(ctx context.Context, topic, key string, payload []byte) error {
	return p.PublishRawWithHeaders(ctx, topic, key, payload, []kafka.Header{
		{
			Key:   "event_type",
			Value: []byte(topic),
		},
		{
			Key:   "schema_version",
			Value: []byte("1"),
		},
	})
}

func (p *Producer) PublishRawWithHeaders(
	ctx context.Context,
	topic, key string,
	payload []byte,
	headers []kafka.Header,
) error {
	w, ok := p.writers[topic]
	if !ok {
		return fmt.Errorf("kafka producer: unknown topic %q", topic)
	}

	msg := kafka.Message{
		Key:     []byte(key),
		Value:   payload,
		Headers: headers,
	}

	if err := w.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka producer: write to %q: %w", topic, err)
	}

	return nil
}

func (p *Producer) PublishDLQ(
	ctx context.Context,
	sourceTopic string,
	msg kafka.Message,
	cause error,
) error {
	event := domain.DeadLetterEvent{
		EventID:         uuid.NewString(),
		SourceTopic:     sourceTopic,
		SourcePartition: msg.Partition,
		SourceOffset:    msg.Offset,
		SourceKey:       string(msg.Key),
		Error:           cause.Error(),
		OriginalValue:   string(msg.Value),
		FailedAt:        time.Now().UTC(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal dlq event: %w", err)
	}

	key := fmt.Sprintf("%s:%d:%d", sourceTopic, msg.Partition, msg.Offset)

	return p.PublishRawWithHeaders(ctx, domain.TopicMatchingDLQ, key, payload, []kafka.Header{
		{
			Key:   "event_type",
			Value: []byte("dead_letter"),
		},
		{
			Key:   "source_topic",
			Value: []byte(sourceTopic),
		},
		{
			Key:   "schema_version",
			Value: []byte("1"),
		},
	})
}

func (p *Producer) PublishRideOfferCreated(ctx context.Context, event domain.RideOfferCreatedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal RideOfferCreatedEvent: %w", err)
	}

	return p.PublishRaw(
		ctx,
		domain.TopicRideOfferCreated,
		event.DriverID,
		payload,
	)
}

func (p *Producer) PublishDriverAssigned(ctx context.Context, event domain.DriverAssignedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal DriverAssignedEvent: %w", err)
	}

	return p.PublishRaw(
		ctx,
		domain.TopicRideDriverAssigned,
		event.RideID,
		payload,
	)
}

func (p *Producer) PublishMatchingFailed(ctx context.Context, event domain.MatchingFailedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal MatchingFailedEvent: %w", err)
	}

	return p.PublishRaw(
		ctx,
		domain.TopicRideMatchingFailed,
		event.RideID,
		payload,
	)
}
