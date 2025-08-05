package main

import (
	"log"
	"net/http"

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

	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Println(err)
	}
}
