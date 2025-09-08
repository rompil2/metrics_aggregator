package main

import (
	"context"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rompil2/metrics_aggregator/internal/config"
	"github.com/rompil2/metrics_aggregator/internal/handler"
	"github.com/rompil2/metrics_aggregator/internal/repository"
	"github.com/rompil2/metrics_aggregator/internal/service"
	"github.com/rompil2/metrics_aggregator/internal/store"
)

func main() {
	var pingHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	var repo service.Repo
	cfg := config.LoadServerConfig(os.Args[1:])
	if cfg.DBConnStr == "" {

		memRepo, err := repository.NewMemStorage()
		if err != nil {
			log.Fatal(err)
		}
		repo = memRepo
		if cfg.FileStoragePath != "" {
			repo, err = store.NewStore(repo, cfg.StoreConfig)
			if err != nil {
				log.Fatal(err)
			}
		}
	} else {

		db, err := sql.Open("pgx", cfg.DBConnStr)
		if err != nil {
			panic(err)
		}
		defer db.Close()

		pingHandler = func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if err = db.PingContext(ctx); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)

		}
	}
	srvc := service.NewMetricService(repo)

	handler := handler.NewHandlerMux(&srvc, template.Must(template.ParseFiles("templates/index.html")))

	handler.Get("/ping", http.HandlerFunc(pingHandler))

	server := &http.Server{
		Addr:    cfg.String(),
		Handler: handler,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("The server is starting at %s with config %#v \n", server.Addr, cfg)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-done
	log.Println("The server is shuting down...")
	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown failed: %v\n", err)
		if err := server.Close(); err != nil {
			log.Fatalf("Forced shutdown failed: %v\n", err)
		}
	}

	log.Println("Server stopped")

}
