package handler

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rompil2/metrics_aggregator/internal/model"
)

type Service interface {
	UpdateMetrics(metric *model.Metrics) error
	GetMetrics(ID string) (model.Metrics, error)
	AllMetrics() ([]model.Metrics, error)
}

type HandlerMux struct {
	chi.Router
	Service Service
	tmpl    *template.Template
}

func NewHandlerMux(service Service, tmpl *template.Template) *HandlerMux {

	h := &HandlerMux{
		Service: service,
		tmpl:    tmpl,
	}
	h.Router = chi.NewRouter()
	h.Use(middleware.RequestID)
	h.Use(middleware.RealIP)
	h.Use(middleware.Logger)
	h.Use(middleware.Recoverer)

	h.Get("/", h.HomePage)
	h.Post("/update/{mtype}/{id}/{value}", h.UpdateMetrics)
	h.Get("/value/{mtype}/{id}", h.GetMetrics)

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

	mtype := chi.URLParam(r, "mtype")
	switch mtype {
	case model.Counter, model.Gauge:
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id := chi.URLParam(r, "id")
	metrics, err := h.Service.GetMetrics(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "plain/text")
	switch metrics.MType {
	case model.Counter:
		fmt.Fprintf(w, "%d", *metrics.Delta)
	case model.Gauge:
		fmt.Fprintf(w, "%f", *metrics.Value)
	}
}

func (h *HandlerMux) UpdateMetrics(w http.ResponseWriter, r *http.Request) {

	ID := chi.URLParam(r, "id")
	MType := chi.URLParam(r, "mtype")
	ValDelta := chi.URLParam(r, "value")

	metricsModel, parseErr := BuildMetrics(MType, ID, ValDelta)

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

func BuildMetrics(mType string, id string, val string) (model.Metrics, error) {

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
