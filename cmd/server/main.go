package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/handler"
	"github.com/rompil2/metrics_aggregator/internal/repository"
	"github.com/rompil2/metrics_aggregator/internal/service"
)

func main() {

	repository, err := repository.NewMemStorage()
	if err != nil {
		log.Fatal(err)
	}
	srvc := service.NewMetricService(repository)

	handler := handler.NewHandlerMux(&srvc)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Println("The server is starting...")
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
