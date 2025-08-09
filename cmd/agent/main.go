package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/agent"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Настройка агента
	collector := agent.NewCollector(1 * time.Second)
	client := agent.NewHTTPClient(1*time.Second, "localhost", 8080)
	agent := agent.New(collector, client)

	// Запуск агента
	go func() {
		if err := agent.Run(ctx); err != nil {
			log.Printf("Agent stopped: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	cancel()
	time.Sleep(1 * time.Second) // Даем время на завершение
}
