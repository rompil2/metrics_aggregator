package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/agent"
	"github.com/rompil2/metrics_aggregator/internal/config"
)

const (
	SIGTERM_CH_SIZE  = 1
	WAIT_BEFORE_QUIT = 1 // in seconds
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Настройка агента
	cfg := config.LoadAgentConfig()
	collector := agent.NewCollector(cfg.PollInterval)
	client := agent.NewHTTPClient(cfg.ReportInterval, cfg.ServerHost, cfg.ServerPort)
	agent := agent.New(collector, client)

	// Запуск агента
	go func() {
		if err := agent.Run(ctx); err != nil {
			log.Printf("Agent stopped: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	stop := make(chan os.Signal, SIGTERM_CH_SIZE)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	cancel()
	time.Sleep(WAIT_BEFORE_QUIT * time.Second) // Даем время на завершение
}
