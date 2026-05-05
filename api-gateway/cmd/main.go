package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/api-gateway/pkg/api"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/api-gateway/pkg/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/gkns/elastic-gpu-telemetry-pipeline/api-gateway/docs"
)

// @title Elastic GPU Telemetry API
// @version 1.0
// @description REST API for querying GPU telemetry data from QuestDB.
// @host localhost:8080
// @BasePath /api/v1
func main() {
	addr := flag.String("addr", ":8080", "Listen address")
	dbConn := flag.String("db", "postgres://admin:quest@localhost:8812/qdb", "QuestDB connection string")
	flag.Parse()

	db, err := store.NewDB(*dbConn)
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}

	a := api.NewAPI(db)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/gpus", a.GetGPUs)
		r.Get("/gpus/{id}/telemetry", a.GetTelemetry)
	})

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	log.Printf("API Gateway listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, r))
}
