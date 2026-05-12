package domain

import "time"

const (
	// Publishing
	TopicRideOfferCreated   = "ride.offer.created"
	TopicRideDriverAssigned = "ride.driver.assigned"
	TopicRideMatchingFailed = "ride.matching.failed"
	// Consuming
	TopicRideRequested     = "ride.requested"
	TopicRideCancelled     = "ride.cancelled"
	TopicRideOfferAccepted = "ride.offer.accepted"
	TopicRideOfferDeclined = "ride.offer.declined"
	// DLQ
	TopicMatchingDLQ = "matching.dlq"
)

// Publish events
type DriverAssignedEvent struct {
	EventID      string    `json:"event_id"`
	RideID       string    `json:"ride_id"`
	DriverID     string    `json:"driver_id"`
	DriverName   string    `json:"driver_name"`
	DriverRating float64   `json:"driver_rating"`
	CarModel     string    `json:"car_model"`
	CarPlate     string    `json:"car_plate"`
	ETASeconds   int       `json:"eta_seconds"`
	AssignedAt   time.Time `json:"assigned_at"`
}

type MatchingFailedEvent struct {
	EventID  string    `json:"event_id"`
	RideID   string    `json:"ride_id"`
	Reason   string    `json:"reason"` // "no_drivers_available" / "timeout"
	FailedAt time.Time `json:"failed_at"`
}

type RideOfferCreatedEvent struct {
	EventID string `json:"event_id"`
	OfferID string `json:"offer_id"`
	RideID  string `json:"ride_id"`

	DriverID string `json:"driver_id"`

	PickupLat     float64 `json:"pickup_lat"`
	PickupLng     float64 `json:"pickup_lng"`
	PickupAddress string  `json:"pickup_address"`

	DestLat     float64 `json:"dest_lat"`
	DestLng     float64 `json:"dest_lng"`
	DestAddress string  `json:"dest_address"`

	EstimatedPriceTiyn int64 `json:"estimated_price_tiyn"`
	ETASeconds         int   `json:"eta_seconds"`

	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Dead letter event is failed kafka event
type DeadLetterEvent struct {
	EventID         string    `json:"event_id"`
	SourceTopic     string    `json:"source_topic"`
	SourcePartition int       `json:"source_partition"`
	SourceOffset    int64     `json:"source_offset"`
	SourceKey       string    `json:"source_key"`
	Error           string    `json:"error"`
	OriginalValue   string    `json:"original_value"`
	FailedAt        time.Time `json:"failed_at"`
}

// Consume events
type RideRequestedEvent struct {
	EventID     string `json:"event_id"`
	RideID      string `json:"ride_id"`
	PassengerID string `json:"passenger_id"`

	PickupLat     float64 `json:"pickup_lat"`
	PickupLng     float64 `json:"pickup_lng"`
	PickupAddress string  `json:"pickup_address,omitempty"`

	DestLat     float64 `json:"dest_lat"`
	DestLng     float64 `json:"dest_lng"`
	DestAddress string  `json:"dest_address,omitempty"`

	CarType            string `json:"car_type"`
	EstimatedPriceTiyn int64  `json:"estimated_price_tiyn"`

	CreatedAt time.Time `json:"created_at"`
}

type RideCancelledEvent struct {
	EventID            string    `json:"event_id"`
	RideID             string    `json:"ride_id"`
	CancelledBy        string    `json:"cancelled_by"` // passenger / driver
	CancellationReason string    `json:"cancellation_reason,omitempty"`
	CancelledAt        time.Time `json:"cancelled_at"`
}

type RideOfferAcceptedEvent struct {
	EventID    string    `json:"event_id"`
	OfferID    string    `json:"offer_id"`
	RideID     string    `json:"ride_id"`
	DriverID   string    `json:"driver_id"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type RideOfferDeclinedEvent struct {
	EventID    string    `json:"event_id"`
	OfferID    string    `json:"offer_id"`
	RideID     string    `json:"ride_id"`
	DriverID   string    `json:"driver_id"`
	Reason     string    `json:"reason"`
	DeclinedAt time.Time `json:"declined_at"`
}
