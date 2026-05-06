package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/config"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
	_ "github.com/gkns/elastic-gpu-telemetry-pipeline/cmd/api-gateway/docs"
)

func main() {
	cfg := config.LoadAPIGatewayConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	ctx := context.Background()
	pgClient, err := questdb.NewPGClient(ctx, cfg.QuestDBPGAddr)
	if err != nil {
		logger.Error("connect to QuestDB", "err", err)
		os.Exit(1)
	}
	defer pgClient.Close()

	h := &Handlers{q: pgClient, pageSizeMax: cfg.PageSizeMax}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/api/v1/gpus", h.ListGPUs)
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)
	r.Get("/healthz", h.HealthCheck)
	r.Get("/swagger/*", httpSwagger.Handler())

	logger.Info("API Gateway listening", "addr", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
