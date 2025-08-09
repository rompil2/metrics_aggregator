package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
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
	tmpl    *template.Template
}

func NewHandlerMux(service Service) *HandlerMux {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	h := &HandlerMux{
		Service: service,
		tmpl:    tmpl,
	}
	h.Handle("/update/", http.StripPrefix("/update/", MiddlewarePostOnly(h.UpdateMetrics)))
	h.Handle("/", http.HandlerFunc(h.HomePage))
	h.Handle("/value/", http.StripPrefix("/value/", MiddlewareGetOnly(h.GetMetrics)))
	return h
}

func (h *HandlerMux) HomePage(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.Service.AllMetrics()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h.tmpl.Execute(w, metrics)
}

func (h *HandlerMux) GetMetrics(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	id := parts[len(parts)-1] // this how get the last element of the slited string, its ID
	metrics, err := h.Service.GetMetrics(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (h *HandlerMux) UpdateMetrics(w http.ResponseWriter, r *http.Request) {

	//parse requests path
	components, err := PathToParse(r.URL.Path)
	if err != nil {
		switch len(components) {
		case 1:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}

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
			return components, fmt.Errorf("too many components in path: %s", path)
		} else {
			return components, fmt.Errorf("not enough components in path: %s", path)
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
			return model.Metrics{}, errors.New("wrong delta format, must be int")
		}
		return model.Metrics{
			ID:    id,
			MType: mType,
			Delta: &val,
		}, err

	case model.Gauge:
		val, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return model.Metrics{}, errors.New("wrong value format, must be float")
		}
		return model.Metrics{
			ID:    id,
			MType: mType,
			Value: &val,
		}, err
	default:
		return model.Metrics{}, errors.New("unknown metrics type")
	}

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

func MiddlewareGetOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//only Get methods are allowed
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprint(w, "Only Get method is allowed")
			return
		}
		next.ServeHTTP(w, r)
	}
}
