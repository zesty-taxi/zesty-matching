package controller

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"github.com/zesty-taxi/zesty-matching/internal/config"
	"github.com/zesty-taxi/zesty-matching/pkg/metrics"
	"github.com/zesty-taxi/zesty-matching/pkg/middlewares"
)

func (h *handler) LoadRoutes(cfg config.Config, m *metrics.Metrics) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.Recoverer)
	router.Use(middleware.RealIP)

	router.Handle("/metrics", promhttp.Handler())

	router.Group(func(r chi.Router) {
		r.Use(otelchi.Middleware(cfg.Telemetry.ServiceName))
		r.Use(middlewares.HeaderGetMiddleware)
		r.Use(middlewares.LoggerMiddleware)
		r.Use(middlewares.MetricsMiddleware(m))
		r.Use(middlewares.GetWithAuthMiddleware)
	})
	return router
}
