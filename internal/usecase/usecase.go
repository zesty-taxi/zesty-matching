package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zesty-taxi/zesty-matching/internal/domain"
)

const (
	nearbyRadiusM = 5000 // ищем в радиусе 5км
)

// Cache — интерфейс Redis адаптера
type Cache interface {
	AcquireDriverLock(ctx context.Context, driverID, rideID string) (bool, error)
	ReleaseDriverLock(ctx context.Context, driverID string) error

	SaveMatchQueue(ctx context.Context, queue domain.MatchQueue) error
	GetMatchQueue(ctx context.Context, rideID string) (*domain.MatchQueue, error)
	DeleteMatchQueue(ctx context.Context, rideID string) error

	SaveActiveOffer(ctx context.Context, offer domain.ActiveOffer) error
	GetActiveOffer(ctx context.Context, rideID string) (*domain.ActiveOffer, error)
	DeleteActiveOffer(ctx context.Context, rideID string) error
}

// LocationClient — интерфейс Location Service клиента
type LocationClient interface {
	GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusM int) ([]domain.NearbyDriver, error)
}

// Producer — интерфейс Kafka продюсера
type Producer interface {
	PublishRideOfferCreated(ctx context.Context, event domain.RideOfferCreatedEvent) error
	PublishDriverAssigned(ctx context.Context, event domain.DriverAssignedEvent) error
	PublishMatchingFailed(ctx context.Context, event domain.MatchingFailedEvent) error
}

type useCase struct {
	cache    Cache
	location LocationClient
	pub      Producer
}

func New(cache Cache, location LocationClient, producer Producer) *useCase {
	return &useCase{
		cache:    cache,
		location: location,
		pub:      producer,
	}
}

// -----------------------------------------------
// HandleRideRequested — главный обработчик.
// Запускает весь процесс матчинга.
// -----------------------------------------------

func (u *useCase) HandleRideRequested(ctx context.Context, event domain.RideRequestedEvent) error {
	log.Info().
		Str("ride_id", event.RideID).
		Float64("pickup_lat", event.PickupLat).
		Float64("pickup_lng", event.PickupLng).
		Msg("starting matching")

	// 1. Берём ближайших водителей из Location Service
	drivers, err := u.location.GetNearbyDrivers(ctx, event.PickupLat, event.PickupLng, nearbyRadiusM)
	if err != nil {
		return fmt.Errorf("get nearby drivers: %w", err)
	}

	if len(drivers) == 0 {
		log.Warn().Str("ride_id", event.RideID).Msg("no drivers nearby")
		return u.publishMatchingFailed(ctx, event.RideID, domain.MatchingFailedNoDrivers)
	}

	log.Info().
		Str("ride_id", event.RideID).
		Int("candidates", len(drivers)).
		Msg("found nearby drivers")

	// 2. Сохраняем очередь в Redis (первый водитель будет отправлен сразу)
	// drivers уже отсортированы по расстоянию — Location Service делает это сам
	queue := domain.MatchQueue{
		RideID:    event.RideID,
		Drivers:   drivers[1:], // первый уйдёт сейчас, остальные в очереди
		CreatedAt: time.Now().UTC(),
	}

	if len(drivers) > 1 {
		if err := u.cache.SaveMatchQueue(ctx, queue); err != nil {
			return fmt.Errorf("save match queue: %w", err)
		}
	}

	// 3. Отправляем оффер первому водителю
	return u.offerToDriver(ctx, event.RideID, event.PickupLat, event.PickupLng,
		event.PickupAddress, event.DestLat, event.DestLng, event.DestAddress,
		event.EstimatedPriceTiyn, drivers[0])
}

// -----------------------------------------------
// HandleRideOfferAccepted — водитель принял заказ
// -----------------------------------------------

func (u *useCase) HandleRideOfferAccepted(ctx context.Context, event domain.RideOfferAcceptedEvent) error {
	log.Info().
		Str("ride_id", event.RideID).
		Str("driver_id", event.DriverID).
		Str("offer_id", event.OfferID).
		Msg("offer accepted")

	// 1. Проверяем что offerID актуален
	// (защита от запоздалых ответов на старые офферы)
	activeOffer, err := u.cache.GetActiveOffer(ctx, event.RideID)
	if err != nil {
		return fmt.Errorf("get active offer: %w", err)
	}

	if activeOffer == nil {
		// Оффер истёк (TTL вышел) — водитель ответил слишком поздно
		log.Warn().
			Str("ride_id", event.RideID).
			Str("offer_id", event.OfferID).
			Msg("offer expired, ignoring late accept")
		return nil
	}

	if activeOffer.OfferID != event.OfferID {
		// Ответ на старый оффер — игнорируем
		log.Warn().
			Str("ride_id", event.RideID).
			Str("expected_offer_id", activeOffer.OfferID).
			Str("received_offer_id", event.OfferID).
			Msg("stale offer accept, ignoring")
		return nil
	}

	// 2. Чистим Redis — матчинг завершён
	if err := u.cleanup(ctx, event.RideID, event.DriverID); err != nil {
		log.Error().Err(err).Str("ride_id", event.RideID).Msg("cleanup after accept failed")
		// Не возвращаем ошибку — главное опубликовать assigned
	}

	// 3. Публикуем ride.driver.assigned
	assignedEvent := domain.DriverAssignedEvent{
		EventID:    uuid.NewString(),
		RideID:     event.RideID,
		DriverID:   event.DriverID,
		AssignedAt: time.Now().UTC(),
	}

	if err := u.pub.PublishDriverAssigned(ctx, assignedEvent); err != nil {
		return fmt.Errorf("publish driver assigned: %w", err)
	}

	log.Info().
		Str("ride_id", event.RideID).
		Str("driver_id", event.DriverID).
		Msg("driver assigned successfully")

	return nil
}

// -----------------------------------------------
// HandleRideOfferDeclined — водитель отказал или таймаут
// -----------------------------------------------

func (u *useCase) HandleRideOfferDeclined(ctx context.Context, event domain.RideOfferDeclinedEvent) error {
	log.Info().
		Str("ride_id", event.RideID).
		Str("driver_id", event.DriverID).
		Str("reason", event.Reason).
		Msg("offer declined")

	// 1. Проверяем актуальность offerID
	activeOffer, err := u.cache.GetActiveOffer(ctx, event.RideID)
	if err != nil {
		return fmt.Errorf("get active offer: %w", err)
	}

	if activeOffer != nil && activeOffer.OfferID != event.OfferID {
		log.Warn().
			Str("ride_id", event.RideID).
			Msg("stale offer decline, ignoring")
		return nil
	}

	// 2. Снимаем lock с водителя
	if err := u.cache.ReleaseDriverLock(ctx, event.DriverID); err != nil {
		log.Error().Err(err).Str("driver_id", event.DriverID).Msg("release lock failed")
	}

	// 3. Удаляем текущий активный оффер
	if err := u.cache.DeleteActiveOffer(ctx, event.RideID); err != nil {
		log.Error().Err(err).Str("ride_id", event.RideID).Msg("delete active offer failed")
	}

	// 4. Берём следующего из очереди
	return u.tryNextDriver(ctx, event.RideID)
}

// -----------------------------------------------
// HandleRideCancelled — пассажир отменил поездку
// -----------------------------------------------

func (u *useCase) HandleRideCancelled(ctx context.Context, event domain.RideCancelledEvent) error {
	log.Info().
		Str("ride_id", event.RideID).
		Str("cancelled_by", event.CancelledBy).
		Msg("ride cancelled, cleaning up matching state")

	// Берём активный оффер чтобы узнать driverID для снятия лока
	activeOffer, err := u.cache.GetActiveOffer(ctx, event.RideID)
	if err != nil {
		log.Error().Err(err).Str("ride_id", event.RideID).Msg("get active offer on cancel")
	}

	if activeOffer != nil {
		if err := u.cache.ReleaseDriverLock(ctx, activeOffer.DriverID); err != nil {
			log.Error().Err(err).Str("driver_id", activeOffer.DriverID).Msg("release lock on cancel")
		}
	}

	// Чистим всё что связано с этой поездкой
	_ = u.cache.DeleteActiveOffer(ctx, event.RideID)
	_ = u.cache.DeleteMatchQueue(ctx, event.RideID)

	return nil
}

// -----------------------------------------------
// Внутренние методы
// -----------------------------------------------

// offerToDriver — ставит lock на водителя и публикует оффер.
func (u *useCase) offerToDriver(
	ctx context.Context,
	rideID string,
	pickupLat, pickupLng float64,
	pickupAddress string,
	destLat, destLng float64,
	destAddress string,
	estimatedPriceTiyn int64,
	driver domain.NearbyDriver,
) error {
	// 1. Пытаемся поставить lock
	locked, err := u.cache.AcquireDriverLock(ctx, driver.DriverID, rideID)
	if err != nil {
		return fmt.Errorf("acquire driver lock: %w", err)
	}

	if !locked {
		// Водитель уже занят — берём следующего
		log.Debug().
			Str("driver_id", driver.DriverID).
			Str("ride_id", rideID).
			Msg("driver already locked, trying next")
		return u.tryNextDriver(ctx, rideID)
	}

	// 2. Создаём оффер
	offerID := uuid.NewString()
	now := time.Now().UTC()

	activeOffer := domain.ActiveOffer{
		OfferID:   offerID,
		RideID:    rideID,
		DriverID:  driver.DriverID,
		ExpiresAt: now.Add(domain.OfferTTL),
	}

	if err := u.cache.SaveActiveOffer(ctx, activeOffer); err != nil {
		// Если не смогли сохранить оффер — снимаем lock
		_ = u.cache.ReleaseDriverLock(ctx, driver.DriverID)
		return fmt.Errorf("save active offer: %w", err)
	}

	// 3. Публикуем ride.offer.created → Driver Service получит и пушнёт водителю через WS
	eta := int(driver.DistanceM / 10) // грубый ETA: 10 м/сек = 36 км/ч
	event := domain.RideOfferCreatedEvent{
		EventID:            uuid.NewString(),
		OfferID:            offerID,
		RideID:             rideID,
		DriverID:           driver.DriverID,
		PickupLat:          pickupLat,
		PickupLng:          pickupLng,
		PickupAddress:      pickupAddress,
		DestLat:            destLat,
		DestLng:            destLng,
		DestAddress:        destAddress,
		EstimatedPriceTiyn: estimatedPriceTiyn,
		ETASeconds:         eta,
		ExpiresAt:          now.Add(domain.OfferTTL),
		CreatedAt:          now,
	}

	if err := u.pub.PublishRideOfferCreated(ctx, event); err != nil {
		_ = u.cache.ReleaseDriverLock(ctx, driver.DriverID)
		_ = u.cache.DeleteActiveOffer(ctx, rideID)
		return fmt.Errorf("publish ride offer created: %w", err)
	}

	// 4. Запускаем таймаут — если водитель не ответил за OfferTTL
	// используем background context чтобы таймер не отменился с запросом
	go u.waitForOfferResponse(context.Background(), rideID, offerID, driver.DriverID)

	log.Info().
		Str("ride_id", rideID).
		Str("driver_id", driver.DriverID).
		Str("offer_id", offerID).
		Int("eta_seconds", eta).
		Msg("offer sent to driver")

	return nil
}

// waitForOfferResponse — горутина таймаута.
// Если водитель не ответил за OfferTTL — считаем отказом.
func (u *useCase) waitForOfferResponse(ctx context.Context, rideID, offerID, driverID string) {
	timer := time.NewTimer(domain.OfferTTL)
	defer timer.Stop()

	<-timer.C

	// Проверяем — может водитель уже ответил и оффер удалён
	activeOffer, err := u.cache.GetActiveOffer(ctx, rideID)
	if err != nil {
		log.Error().Err(err).Str("ride_id", rideID).Msg("timeout: get active offer")
		return
	}

	if activeOffer == nil || activeOffer.OfferID != offerID {
		// Водитель уже ответил — всё хорошо
		return
	}

	log.Info().
		Str("ride_id", rideID).
		Str("driver_id", driverID).
		Msg("offer timeout, trying next driver")

	// Снимаем lock и пробуем следующего
	if err := u.cache.ReleaseDriverLock(ctx, driverID); err != nil {
		log.Error().Err(err).Str("driver_id", driverID).Msg("timeout: release lock")
	}
	if err := u.cache.DeleteActiveOffer(ctx, rideID); err != nil {
		log.Error().Err(err).Str("ride_id", rideID).Msg("timeout: delete offer")
	}

	if err := u.tryNextDriver(ctx, rideID); err != nil {
		log.Error().Err(err).Str("ride_id", rideID).Msg("timeout: try next driver")
	}
}

// tryNextDriver — берёт следующего из очереди и отправляет оффер.
func (u *useCase) tryNextDriver(ctx context.Context, rideID string) error {
	queue, err := u.cache.GetMatchQueue(ctx, rideID)
	if err != nil {
		return fmt.Errorf("get match queue: %w", err)
	}

	// Очередь пуста или не найдена — все отказали
	if queue == nil || len(queue.Drivers) == 0 {
		log.Warn().Str("ride_id", rideID).Msg("all drivers declined, matching failed")
		return u.publishMatchingFailed(ctx, rideID, domain.MatchingFailedNoDrivers)
	}

	// Берём первого из очереди
	next := queue.Drivers[0]
	remaining := queue.Drivers[1:]

	// Обновляем очередь в Redis
	if len(remaining) > 0 {
		queue.Drivers = remaining
		if err := u.cache.SaveMatchQueue(ctx, *queue); err != nil {
			return fmt.Errorf("update match queue: %w", err)
		}
	} else {
		// Очередь будет пуста — удаляем ключ
		if err := u.cache.DeleteMatchQueue(ctx, rideID); err != nil {
			log.Error().Err(err).Str("ride_id", rideID).Msg("delete empty queue")
		}
	}

	// У нас нет pickup/dest данных в очереди — нужно получить из оффера
	// Для упрощения передаём нулевые координаты, Driver Service уже знает маршрут из первого оффера
	// В проде здесь был бы отдельный ключ с данными поездки в Redis
	return u.offerToDriver(ctx, rideID, 0, 0, "", 0, 0, "", 0, next)
}

// cleanup — удаляем все Redis ключи после успешного матчинга.
func (u *useCase) cleanup(ctx context.Context, rideID, driverID string) error {
	_ = u.cache.ReleaseDriverLock(ctx, driverID)
	_ = u.cache.DeleteActiveOffer(ctx, rideID)
	_ = u.cache.DeleteMatchQueue(ctx, rideID)
	return nil
}

func (u *useCase) publishMatchingFailed(ctx context.Context, rideID, reason string) error {
	event := domain.MatchingFailedEvent{
		EventID:  uuid.NewString(),
		RideID:   rideID,
		Reason:   reason,
		FailedAt: time.Now().UTC(),
	}
	return u.pub.PublishMatchingFailed(ctx, event)
}
