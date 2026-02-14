package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/agent"
	"github.com/rompil2/metrics_aggregator/internal/config"
	"github.com/rompil2/metrics_aggregator/internal/crypto"
	"github.com/rompil2/metrics_aggregator/internal/model"
)

const (
	sigterChSize   = 1
	waitBeforeQuit = 1 // in seconds
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
	cfg := config.LoadAgentConfig(os.Args[1:])
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publicKey, err := crypto.LoadPublicKey(cfg.PublicKeyPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Public key file not found: %v", err)
		} else {
			log.Fatalf("Error loading public key: %v", err)
		}
	}

	// Настройка HTTP-агента
	httpCollector := agent.NewCollector(cfg.PollInterval)
	httpClient := agent.NewHTTPClient(cfg.ReportInterval, cfg.SocketConfig.Host, uint(cfg.SocketConfig.Port), true, cfg.HashConfig.String(), cfg.RateLimit, publicKey)
	httpAgent := agent.New(httpCollector, httpClient)

	// Запуск HTTP-агента
	go func() {
		if err := httpAgent.Run(ctx); err != nil {
			log.Printf("HTTP Agent stopped: %v", err)
		}
	}()

	// Настройка gRPC-клиента (если адрес задан)
	var grpcClient *agent.GRPCClient
	if cfg.GRPCAddr != "" {
		grpcClient, err = agent.NewGRPCClient(cfg.GRPCAddr)
		if err != nil {
			log.Printf("Warning: failed to create gRPC client: %v", err)
		} else {
			defer grpcClient.Close()

			// Запуск gRPC-отправки метрик
			go func() {
				ticker := time.NewTicker(cfg.ReportInterval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						// Сбор метрик
						metrics := httpCollector.Poll()
						metricsSlice := convertMetricsMapToSlice(metrics)

						// Отправка через gRPC
						if err := grpcClient.SendMetrics(ctx, metricsSlice); err != nil {
							log.Printf("Failed to send metrics via gRPC: %v", err)
						}
					}
				}
			}()
		}
	}

	// Ожидание сигнала завершения
	stop := make(chan os.Signal, sigterChSize)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-stop

	log.Println("Shutting down...")
	cancel()
	time.Sleep(waitBeforeQuit * time.Second) // Даем время на завершение
}

// convertMetricsMapToSlice преобразует map[string]any в []model.Metrics
func convertMetricsMapToSlice(metrics map[string]any) []model.Metrics {
	result := make([]model.Metrics, 0, len(metrics))

	for key, value := range metrics {
		var m model.Metrics
		m.ID = key

		switch val := value.(type) {
		case int64:
			m.MType = model.Counter
			m.Delta = &val
		case float64:
			m.MType = model.Gauge
			m.Value = &val
		}

		if m.Delta != nil || m.Value != nil {
			result = append(result, m)
		}
	}

	return result
}
