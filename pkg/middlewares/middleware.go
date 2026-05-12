package middlewares

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/zesty-taxi/zesty-matching/pkg/metrics"
	"github.com/zesty-taxi/zesty-matching/pkg/render"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	RequestIDKey     contextKey = "request_id"
	CorrelationIDKey contextKey = "correlation_id"
	UserIDKey        contextKey = "user_id"
	UserRoleKey      contextKey = "user_role"
)

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		reqID, _ := r.Context().Value(RequestIDKey).(string)
		correlationID, _ := r.Context().Value(CorrelationIDKey).(string)

		spanCtx := trace.SpanFromContext(r.Context()).SpanContext()

		logCtx := log.With().
			Str("request_id", reqID).
			Str("trace_id", spanCtx.TraceID().String()).
			Str("span_id", spanCtx.SpanID().String()).
			Logger()

		if correlationID != "" {
			logCtx = logCtx.With().Str("correlation_id", correlationID).Logger()
		}

		ctx := logCtx.WithContext(r.Context())

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r.WithContext(ctx))

		event := logEvent(logCtx, ww.Status())
		event.
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Int("bytes", ww.BytesWritten()).
			Dur("duration_ms", time.Since(start)).
			Msg("request handled")
	})
}

func logEvent(logger zerolog.Logger, status int) *zerolog.Event {
	switch {
	case status >= 500:
		return logger.Error()
	case status >= 400:
		return logger.Warn()
	default:
		return logger.Info()
	}
}

func GetWithAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			render.JSON(w, http.StatusUnauthorized, struct {
				Message string
			}{
				Message: "unauthorized: missing user id",
			})
			return
		}

		userRole := r.Header.Get("X-User-Role")

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, userRole)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func HeaderGetMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		w.Header().Set("X-Request-ID", reqID)

		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)

		cid := r.Header.Get("X-Correlation-ID")
		if cid != "" {
			w.Header().Set("X-Correlation-ID", cid)
			ctx = context.WithValue(ctx, CorrelationIDKey, cid)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func MetricsMiddleware(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			m.ActiveConnections.Add(r.Context(), 1)

			next.ServeHTTP(ww, r)

			ctx := context.WithoutCancel(r.Context())

			defer m.ActiveConnections.Add(ctx, -1)

			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = "unknown"
			}

			attrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.route", routePattern),
				attribute.Int("http.status_code", ww.Status()),
			}

			m.HTTPRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
			m.HTTPRequestDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		})
	}
}
