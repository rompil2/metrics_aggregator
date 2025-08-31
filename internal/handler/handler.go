package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rompil2/metrics_aggregator/internal/logger"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/internal/service"
	"github.com/rs/zerolog"
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
	// h.Use(middleware.Logger) // It is a logger from the chi package. It is based on log\slog
	h.Use(NaiveLoggerMiddleware)
	h.Use(middleware.Recoverer)

	h.Get("/", h.HomePage)
	h.Post("/update/", h.UpdateWithJSON)
	h.Post("/update/{mtype}/{id}/{value}", h.UpdateMetrics)
	h.Post("/value/", h.GetMetricsJSON)
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
		fmt.Fprint(w, strconv.FormatFloat(*metrics.Value, 'f', -1, 64))
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
	w.WriteHeader(http.StatusOK)
}

func (h *HandlerMux) UpdateWithJSON(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	log.Debug().Msg("Process updating a metrics")

	var metricsModel model.Metrics
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	if err := decoder.Decode(&metricsModel); err != nil {
		log.Error().Err(err).Msg("cannot parse the request")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Validate MetricModel
	if err := validateMetricsUpdate(&metricsModel); err != nil {
		log.Error().Err(err).Msg("validation failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.Service.UpdateMetrics(&metricsModel)
	if err != nil {
		h.handleUpdateError(w, err, metricsModel.ID, log)
		return
	}

	log.Debug().Str("id", metricsModel.ID).Msg("metric updated")
	w.WriteHeader(http.StatusOK)
}

func (h *HandlerMux) handleUpdateError(w http.ResponseWriter, err error, id string, log zerolog.Logger) {
	if errors.Is(err, service.ErrMetricCreated) {
		log.Info().Str("id", id).Msg("metric created")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "Metric %s created", id)
		return
	}

	log.Error().Err(err).Str("id", id).Msg("update failed")
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func (h *HandlerMux) GetMetricsJSON(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	log.Info().Msg("Process requesting a metrics")

	// It was not in requiremetns but might be usefull
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var metricsModel model.Metrics
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	if err := decoder.Decode(&metricsModel); err != nil {
		log.Error().Err(err).Msg("cannot parse the request")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// validate metrics
	if err := validateMetricsGet(&metricsModel); err != nil {
		log.Error().Err(err).Msg("validation failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	metrics, err := h.Service.GetMetrics(metricsModel.ID)
	if err != nil {
		errMsg := fmt.Sprintf("cannot find metrics with ID: %s", metricsModel.ID)
		log.Error().Err(err).Msg(errMsg)
		http.Error(w, errMsg, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Error().Err(err).Msg("json encode error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("the metrics is returned back")
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

func validateMetricsGet(metrics *model.Metrics) error {
	if metrics.ID == "" {
		return errors.New("ID must be set")
	}
	if metrics.MType != model.Counter && metrics.MType != model.Gauge {
		return fmt.Errorf("unknown metrics type: %s", metrics.MType)
	}
	return nil

}

func validateMetricsUpdate(metrics *model.Metrics) error {
	if err := validateMetricsGet(metrics); err != nil {
		return err
	}
	if metrics.MType == model.Counter && metrics.Delta == nil {
		return errors.New("delta must be set for counter")
	}
	if metrics.MType == model.Gauge && metrics.Value == nil {
		return errors.New("value must be set for gauge")
	}
	return nil
}
