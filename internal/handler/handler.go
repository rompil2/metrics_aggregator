package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

type Service interface {
	UpdateMetrics(metric *model.Metrics) error
	GetMetrics(ID string) (model.Metrics, error)
	AllMetrics() ([]model.Metrics, error)
}

type HandlerMux struct {
	http.ServeMux
	Service Service
}

func NewHandlerMux(service Service) *HandlerMux {
	h := &HandlerMux{
		Service: service,
	}
	h.HandleFunc("/update/", MiddlewareRemoveUpdateFromPath(MiddlewarePostOnly(h.UpdateMetrics)))
	//TODO: add other handlers
	return h
}

func MiddlewarePostOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//only Post methods are allowed
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprint(w, "Only POST method is allowed")
			return
		}
		next.ServeHTTP(w, r)
	}
}

func MiddlewareRemoveUpdateFromPath(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathToParse := strings.TrimPrefix(r.URL.Path, "/update/")
		r.URL.Path = pathToParse
		next.ServeHTTP(w, r)
	}
}

func (h *HandlerMux) UpdateMetrics(w http.ResponseWriter, r *http.Request) {

	//parse requests path
	components, err := PathToParse(r.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, err.Error())
		return
	}

	metricsModel, parseErr := BuildMetrics(components)

	if parseErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, string(parseErr.Error()))
		return
	}

	if err := h.Service.UpdateMetrics(&metricsModel); err != nil {
		// If error is "Unknown metrics ID, created the new one" - then it's a new metrics
		if strings.Contains(err.Error(), "created the new one") {
			//Everything is ok, new metrics was added
			fmt.Fprintf(w, "A new metrics %s was added.", metricsModel.ID)
		}
		return
	}
}

func PathToParse(path string) ([]string, error) {
	if path == "" {
		return nil, errors.New("path is empty")
	}
	// TODO: check path
	components := strings.Split(strings.Trim(path, "/"), "/")
	if len(components) != 3 {
		if len(components) > 3 {
			return nil, fmt.Errorf("Too many components in path: %s", path)
		} else {
			return nil, fmt.Errorf("Not enough componentsin path: %s", path)
		}
	}
	return components, nil
}

func BuildMetrics(components []string) (model.Metrics, error) {
	mType := components[0]
	id := components[1]
	val := components[2]
	switch mType {
	case model.Counter:
		val, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return model.Metrics{}, errors.New("Wrong delta format, must be int")
		}
		return model.Metrics{
			ID:    id,
			MType: mType,
			Delta: &val,
		}, err

	case model.Gauge:
		val, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return model.Metrics{}, errors.New("Wrong value format, must be float")
		}
		return model.Metrics{
			ID:    id,
			MType: mType,
			Value: &val,
		}, err
	default:
		return model.Metrics{}, errors.New("Unknown metrics type")
	}

}
