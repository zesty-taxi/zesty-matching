package domain

import "time"

const (
	// Publishing
	TopicRideDriverAssigned = "ride.driver.assigned"
	TopicRideMatchingFailed = "ride.matching.failed"
	// Consuming
	TopicRideRequested     = "ride.requested"
	TopicRideCancelled     = "ride.cancelled"
	TopicRideOfferAccepted = "ride.offer.accepted"
	TopicRideOfferDeclined = "ride.offer.declined"
	// DLQ
	TopicRideDLQ = "ride.dlq"
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
