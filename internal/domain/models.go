package domain

import "time"

const (
	OfferTTL        = 18 * time.Second
	DriverLockTTL   = 20 * time.Second
	OfferVisibleSec = 15
	MatchingTimeout = 2 * time.Minute
)

// NearbyDriver — водитель из Location Service
type NearbyDriver struct {
	DriverID  string    `json:"driver_id"`
	RideID    string    `json:"ride_id,omitempty"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Heading   int       `json:"heading"`
	SpeedKmh  int       `json:"speed_kmh"`
	UpdatedAt time.Time `json:"updated_at"`
	DistanceM float64   `json:"distance_m"`
}

// ActiveOffer — текущий активный оффер для поездки.
// Хранится в Redis пока водитель думает.
type ActiveOffer struct {
	OfferID   string    `json:"offer_id"`
	RideID    string    `json:"ride_id"`
	DriverID  string    `json:"driver_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MatchQueue — очередь кандидатов для поездки.
// Хранится в Redis. Когда водитель отказал — берём следующего.
type MatchQueue struct {
	RideID    string         `json:"ride_id"`
	Drivers   []NearbyDriver `json:"drivers"`
	CreatedAt time.Time      `json:"created_at"`
}
