-- +goose Up
-- +goose StatementBegin

CREATE TYPE match_response AS ENUM (
    'ACCEPTED',
    'DECLINED',
    'TIMEOUT'       
);

-- -----------------------------------------------
-- MATCH ATTEMPTS
-- Логируем каждую попытку предложить заказ водителю.
-- Нужно для аналитики: сколько водителей отказали,
-- среднее время поиска, и т.д.
-- Основная логика матчинга живёт в Redis (TTL, locks).
-- Эта таблица — только историческая запись.
-- -----------------------------------------------
CREATE TABLE match_attempts (
    attempt_id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    ride_id uuid NOT NULL,
    driver_id uuid NOT NULL,
    -- Расстояние до точки подачи в момент предложения
    distance_meters INT,
    offered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ,
    response match_response,
    -- Порядковый номер попытки для этой поездки (1й водитель, 2й и т.д.)
    attempt_number SMALLINT NOT NULL DEFAULT 1
);

CREATE INDEX idx_match_attempts_ride_id ON match_attempts (ride_id);

CREATE INDEX idx_match_attempts_driver_id ON match_attempts (driver_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_match_attempts_driver_id;

DROP INDEX IF EXISTS idx_match_attempts_ride_id;

DROP TABLE IF EXISTS match_attempts;

DROP TYPE IF EXISTS match_response;

-- +goose StatementEnd