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
	"github.com/rompil2/metrics_aggregator/internal/repository/dbstore"
	"github.com/rompil2/metrics_aggregator/internal/repository/filestore"
	"github.com/rompil2/metrics_aggregator/internal/repository/memstore"
	"github.com/rompil2/metrics_aggregator/internal/service"
)

func main() {
	var (
		repo service.Repo
	)
	cfg := config.LoadServerConfig(os.Args[1:])

	if cfg.DBConnStr == "" {
		// Connecition string is not set
		memRepo := memstore.NewMemStore()

		repo = memRepo
		if cfg.FileStoragePath != "" {
			filerepo, err := filestore.NewFileStore(repo, cfg.StoreConfig)
			if err != nil {
				log.Fatal(err)
			}
			repo = filerepo

		}
	} else {
		// There is some connection string
		db, err := sql.Open("pgx", cfg.DBConnStr)
		if err != nil {
			log.Fatal(err)
		}
		dbRepo, err := dbstore.NewDBStore(db)
		if err != nil {
			log.Fatal(err)
		}
		defer dbRepo.Close()

		repo = dbRepo
	}

	srvc := service.NewMetricService(repo)

	handler := handler.NewHandlerMux(srvc, template.Must(template.ParseFiles("templates/index.html")))

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
