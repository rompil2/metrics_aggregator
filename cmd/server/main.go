package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rompil2/metrics_aggregator/internal/config"
	"github.com/rompil2/metrics_aggregator/internal/crypto"
	"github.com/rompil2/metrics_aggregator/internal/handler"
	"github.com/rompil2/metrics_aggregator/internal/repository/dbstore"
	"github.com/rompil2/metrics_aggregator/internal/repository/filestore"
	"github.com/rompil2/metrics_aggregator/internal/repository/memstore"
	"github.com/rompil2/metrics_aggregator/internal/server"
	"github.com/rompil2/metrics_aggregator/internal/service"
)

const (
	PathToTemplate = "templates/index.html"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

func main() {

	printBuildInfo()

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

	privateKey, err := crypto.LoadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		log.Fatalf("Error loading private key: %v", err)
	}

	srvc := service.NewMetricService(repo)
	handler := handler.NewHandlerMux(
		srvc,
		template.Must(template.ParseFiles(PathToTemplate)),
		cfg.HashConfig.String(),
		cfg.AuditFile.String(),
		cfg.AuditURL.String(),
		privateKey,
		cfg.TrustedSubnet,
	)
	httpServer := &http.Server{
		Addr:    cfg.SocketConfig.String(),
		Handler: handler,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	grpcServerDone := make(chan struct{})
	go func() {
		if err := server.StartGRPCServer(cfg.GRPCAddr, cfg.TrustedSubnet, srvc); err != nil {
			log.Fatal(err)
		}
		close(grpcServerDone)
	}()

	go func() {
		log.Printf("The server is starting at %s with config %#v \n", httpServer.Addr, cfg)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-done
	log.Println("The server is shuting down...")
	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown failed: %v\n", err)
		if err := httpServer.Close(); err != nil {
			log.Fatalf("Forced shutdown failed: %v\n", err)
		}
	}

	select {
	case <-grpcServerDone:
		log.Println("GRPC server shutdown gracefully")
	case <-time.After(5 * time.Second):
		log.Println("GRPC server shutdown timed out")
	}

	log.Println("Server stopped")
}
