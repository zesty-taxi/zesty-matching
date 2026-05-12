package usecase

import (
	"context"

	"github.com/zesty-taxi/zesty-matching/internal/domain"
)

type Producer interface {
	PublishRaw(ctx context.Context, topic, key string, payload []byte) error
}
type useCase struct {
	pub Producer
}

func New(producer Producer) *useCase {
	return &useCase{
		pub: producer,
	}
}

func (u *useCase) HandleRideRequested(ctx context.Context, event domain.RideRequestedEvent) error
func (u *useCase) HandleRideCancelled(ctx context.Context, event domain.RideCancelledEvent) error
func (u *useCase) HandleRideOfferAccepted(ctx context.Context, event domain.RideOfferAcceptedEvent) error
func (u *useCase) HandleRideOfferDeclined(ctx context.Context, event domain.RideOfferDeclinedEvent) error
