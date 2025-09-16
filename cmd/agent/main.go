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
	sigterChSize   = 1
	waitBeforeQuit = 1 // in seconds
)

func main() {

	cfg := config.LoadAgentConfig(os.Args[1:])
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Настройка агента
	collector := agent.NewCollector(cfg.PollInterval)
	client := agent.NewHTTPClient(cfg.ReportInterval, cfg.Host, cfg.Port, true)
	agent := agent.New(collector, client)

	// Запуск агента
	go func() {
		if err := agent.Run(ctx); err != nil {
			log.Printf("Agent stopped: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	stop := make(chan os.Signal, sigterChSize)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	cancel()
	time.Sleep(waitBeforeQuit * time.Second) // Даем время на завершение
}
