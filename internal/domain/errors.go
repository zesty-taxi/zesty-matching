package domain

import "errors"

var (
	ErrUnknownVehicleType    = errors.New("unknown vehicle type")
	ErrNotFound              = errors.New("not found")
	ErrForbidden             = errors.New("forbidden")
	ErrInvalidToken          = errors.New("invalid token")
	ErrExpiredToken          = errors.New("token has expired")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrNoRows                = errors.New("Row not found")
	ErrAlreadyExists         = errors.New("data already exists")
	ErrConnectionNotFound    = errors.New("connection not found")
	ErrConnectionClosed      = errors.New("connection is closed")
	ErrInvalidJSON           = errors.New("invalid JSON body")
	ErrInvalidInput          = errors.New("invalid input")
	ErrInvalidRideStatus     = errors.New("invalid ride status")
	ErrKeyDoesNotExist       = errors.New("key does not exist")
	ErrOSRMUnavailable       = errors.New("OSRM is unavailable")
	ErrFareExpired           = errors.New("fare has expired, please request new estimate")
	ErrFareNotFound          = errors.New("fare not found")
	ErrRideCompleted         = errors.New("ride already completed")
	ErrMissingIdempotencyKey = errors.New("missing idempotency key")
	ErrRideNotArrived        = errors.New("driver has not arrived yet")
	ErrDriverNotAssigned     = errors.New("driver is not assigned to this ride")
	ErrRideCancelled         = errors.New("ride already cancelled")
	ErrRideInProgress        = errors.New("ride already started")
)

type PermanentError struct {
	Err error
}

func (e PermanentError) Error() string {
	return e.Err.Error()
}

func (e PermanentError) Unwrap() error {
	return e.Err
}

func Permanent(err error) error {
	return PermanentError{Err: err}
}

func IsPermanent(err error) bool {
	var permanentErr PermanentError
	return errors.As(err, &permanentErr)
}
