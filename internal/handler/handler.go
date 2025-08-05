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
			fmt.Fprint(w, "Only POST method is allowed")
			w.WriteHeader(http.StatusMethodNotAllowed)
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
	if len(components) != 3 {
		if len(components) > 3 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Too many components")

			return
		} else {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Not enough components")
			return
		}
	}

	metricsModel := model.Metrics{
		ID:    components[1],
		MType: components[0],
		Delta: nil,
		Value: nil,
	}
	{
		var parseErr error
		var delta int64
		var value float64
		switch components[0] {
		// Only known types of metrics are allowed
		case model.Counter:
			delta, parseErr = strconv.ParseInt(components[2], 10, 64)
			if parseErr != nil {
				break
			}
			metricsModel.Delta = &delta
		case model.Gauge:
			value, parseErr = strconv.ParseFloat(components[2], 64)
			if parseErr != nil {
				break
			}
			metricsModel.Value = &value
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Unknown metrics type: %s", components[0])
			return
		}
		if parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, string(parseErr.Error()))
			return
		}
	}
	if err := h.Service.UpdateMetrics(&metricsModel); err != nil {
		// If error is "Unknown metrics ID, create a new one" - then it's a new metrics
		if strings.Contains(err.Error(), "Unknown metrics ID") {
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
	components := strings.Split(path, "/")
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
