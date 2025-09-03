package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/config"
	"github.com/rompil2/metrics_aggregator/internal/handler"
	"github.com/rompil2/metrics_aggregator/internal/repository"
	"github.com/rompil2/metrics_aggregator/internal/service"
	"github.com/rompil2/metrics_aggregator/internal/store"
)

func main() {
	cfg := config.LoadServerConfig(os.Args[1:])

	repository, err := repository.NewMemStorage()
	if err != nil {
		log.Fatal(err)
	}
	store, err := store.NewStore(repository, cfg.StoreConfig)
	if err != nil {
		log.Fatal(err)
	}

	srvc := service.NewMetricService(store)

	handler := handler.NewHandlerMux(&srvc, template.Must(template.ParseFiles("templates/index.html")))

	server := &http.Server{
		Addr:    cfg.String(),
		Handler: handler,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("The server is starting at %s\n", server.Addr)
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
