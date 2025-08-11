package main

import (
	"context"
	"flag"
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

	socket := new(config.NetAddress)
	socket.Host = "localhost"
	socket.Port = 8080 // the default port for the server

	flag.Var(socket, "a", "-a=<host>:<port>")

	pollInterval := flag.Int64("p", 2, "polling Interval in sec")
	reportInterval := flag.Int("r", 10, "report Interval in sec")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Настройка агента
	collector := agent.NewCollector(time.Duration(*pollInterval) * time.Second)
	client := agent.NewHTTPClient(time.Duration(*reportInterval)*time.Second, socket.Host, uint(socket.Port))
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
