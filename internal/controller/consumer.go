package controller

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"github.com/zesty-taxi/zesty-matching/internal/domain"
)

type UseCaseHandle interface {
	HandleRideRequested(ctx context.Context, event domain.RideRequestedEvent) error
	HandleRideCancelled(ctx context.Context, event domain.RideCancelledEvent) error
	HandleRideOfferAccepted(ctx context.Context, event domain.RideOfferAcceptedEvent) error
	HandleRideOfferDeclined(ctx context.Context, event domain.RideOfferDeclinedEvent) error
}

type EventPublisher interface {
	PublishDLQ(
		ctx context.Context,
		sourceTopic string,
		msg kafka.Message,
		cause error,
	) error
}

type Consumer struct {
	readers map[string]*kafka.Reader
	uc      UseCaseHandle
	pub     EventPublisher
}

func NewConsumer(brokers []string, groupID string, uc UseCaseHandle, publisher EventPublisher) *Consumer {
	makeReader := func(topic string) *kafka.Reader {
		return kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			GroupID:        groupID,
			Topic:          topic,
			MinBytes:       1,
			MaxBytes:       1 << 20, // 1MB
			CommitInterval: 0,
		})
	}

	return &Consumer{
		readers: map[string]*kafka.Reader{
			domain.TopicRideRequested:     makeReader(domain.TopicRideRequested),
			domain.TopicRideCancelled:     makeReader(domain.TopicRideCancelled),
			domain.TopicRideOfferAccepted: makeReader(domain.TopicRideOfferAccepted),
			domain.TopicRideOfferDeclined: makeReader(domain.TopicRideOfferDeclined),
		},
		uc:  uc,
		pub: publisher,
	}
}

// Start func starts goroutines for all topics
func (c *Consumer) Start(ctx context.Context) {
	for topic, reader := range c.readers {
		go c.consume(ctx, topic, reader)
	}
}

// Close
func (c *Consumer) Close() {
	for topic, r := range c.readers {
		if err := r.Close(); err != nil {
			log.Warn().Err(err).Str("topic", topic).Msg("failed to close consumer")
		}
	}
}

// consume is consuming one topic
func (c *Consumer) consume(ctx context.Context, topic string, reader *kafka.Reader) {
	log.Info().Str("topic", topic).Msg("kafka consumer started")

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Info().Str("topic", topic).Msg("kafka consumer stopped")
				return
			}

			log.Error().
				Err(err).
				Str("topic", topic).
				Msg("kafka fetch error")

			continue
		}

		if err := c.dispatch(ctx, topic, msg); err != nil {
			log.Error().
				Err(err).
				Int("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Str("key", string(msg.Key)).
				Msg("kafka dispatch error")

			if domain.IsPermanent(err) {
				if dlqErr := c.pub.PublishDLQ(ctx, topic, msg, err); dlqErr != nil {
					log.Error().
						Err(dlqErr).
						Msg("failed to publish message to dlq")

					continue
				}

				if err := reader.CommitMessages(ctx, msg); err != nil {
					log.Error().Err(err).Msg("commit after dlq failed")
				}

				continue
			}

			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Error().
				Err(err).
				Str("topic", topic).
				Int("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Msg("kafka commit error")
			continue
		}
	}
}

// dispatch — routing topics
func (c *Consumer) dispatch(ctx context.Context, topic string, msg kafka.Message) error {
	switch topic {
	case domain.TopicRideRequested:
		return c.handleRideRequested(ctx, msg.Value)
	case domain.TopicRideCancelled:
		return c.handleRideCancelled(ctx, msg.Value)
	case domain.TopicRideOfferAccepted:
		return c.handleRideOfferAccepted(ctx, msg.Value)
	case domain.TopicRideOfferDeclined:
		return c.handleRideOfferDeclined(ctx, msg.Value)
	default:
		return fmt.Errorf("unknown topic: %s", topic)
	}
}

func (c *Consumer) handleRideRequested(ctx context.Context, payload []byte) error {
	var event domain.RideRequestedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return domain.Permanent(fmt.Errorf("unmarshal RideRequestedEvent: %w", err))
	}

	if event.EventID == "" {
		return domain.Permanent(fmt.Errorf("invalid RideRequestedEvent: empty event_id"))
	}

	if event.RideID == "" {
		return domain.Permanent(fmt.Errorf("invalid RideRequestedEvent: empty ride_id"))
	}

	// TO DO: more validate

	log.Info().
		Str("event_id", event.EventID).
		Str("ride_id", event.RideID).
		Msg("ride requested")

	return c.uc.HandleRideRequested(ctx, event)
}

func (c *Consumer) handleRideCancelled(ctx context.Context, payload []byte) error {
	var event domain.RideCancelledEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return domain.Permanent(fmt.Errorf("unmarshal RideCancelledEvent: %w", err))
	}

	// TO DO Validate and log
	return c.uc.HandleRideCancelled(ctx, event)
}

func (c *Consumer) handleRideOfferAccepted(ctx context.Context, payload []byte) error {
	var event domain.RideOfferAcceptedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return domain.Permanent(fmt.Errorf("unmarshal RideOfferAcceptedEvent: %w", err))
	}

	// TO DO Validate and log

	return c.uc.HandleRideOfferAccepted(ctx, event)
}

func (c *Consumer) handleRideOfferDeclined(ctx context.Context, payload []byte) error {
	var event domain.RideOfferDeclinedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return domain.Permanent(fmt.Errorf("unmarshal RideOfferDeclinedEvent: %w", err))
	}

	// TO DO Validate and log

	return c.uc.HandleRideOfferDeclined(ctx, event)
}
