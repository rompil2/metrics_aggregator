//nolint:bodyclose
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rompil2/metrics_aggregator/internal/mocks"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestBuildMetrics(t *testing.T) {
	type args struct {
		mtype string
		id    string
		val   string
	}
	tests := []struct {
		name    string
		args    args
		want    model.Metrics
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"Positive Test. Counter.",
			args{"counter", "cpu", "0"},
			model.Metrics{
				MType: model.Counter,
				ID:    "cpu",
				Delta: new(int64),
			},
			false,
		}, {
			"Positive Test. Gauge.",
			args{"gauge", "memory", "0.0"},
			model.Metrics{
				MType: model.Gauge,
				ID:    "memory",
				Value: new(float64),
			},
			false,
		}, {
			"Negative Test. Counter. Wrong data format.",
			args{"counter", "cpu", "C"},
			model.Metrics{},
			true,
		}, {
			"Negative Test. Gauge. Wrong data format.",
			args{"gauge", "memory", "G"},
			model.Metrics{},
			true,
		}, {
			"Negative Test. Unknown type.",
			args{"gouge", "memory", "G"},
			model.Metrics{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildMetrics(tt.args.mtype, tt.args.id, tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandlerMux_UpdatePost(t *testing.T) {

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errCode int
	}{
		// TODO: Add test cases.
		{
			"Positive test.", "/update/counter/cpu/1", false, http.StatusOK,
		}, {
			"Negative test. Not enought arguments.", "/update/counter/cpu/", true, http.StatusNotFound,
		}, {
			"Negative test. Wrong metrics type.", "/update/caunter/cpu/1", true, http.StatusBadRequest,
		}, {
			"Negative test. Wrong data format", "/update/counter/cpu/t", true, http.StatusBadRequest,
		}, {
			"Negative test. Without counter ID", "/update/counter/", true, http.StatusNotFound,
		}, {
			"Negative test. Without gauge ID", "/update/gauge/", true, http.StatusNotFound,
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlerMux(mockService, nil, "", "", "", nil, "")
			if !tt.wantErr {
				mockService.EXPECT().
					UpdateMetrics(gomock.Any()).
					Return(fmt.Errorf("Unknown metrics ID, created the new one")).
					Times(1)
			}
			r := httptest.NewRequest(http.MethodPost, tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if tt.wantErr {
				assert.Equal(t, tt.errCode, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestHandlerMux_ValueGet(t *testing.T) {

	testMetrics := model.Metrics{
		ID:    "cpu",
		Delta: new(int64),
	}
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errCode int
	}{
		// TODO: Add test cases.
		{
			"Positive test.", "/value/counter/cpu", false, http.StatusOK,
		}, {
			"Negative test. Without counter ID.", "/value/counter", true, http.StatusNotFound,
		}, {
			"Negative test. Unknow ID", "/value/counter/memory", true, http.StatusNotFound,
			// }, {
			// "Negative test. Empty path", "/value/", true, http.StatusNotFound, // not actual any more.
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlerMux(mockService, nil, "", "", "", nil, "")
			if tt.wantErr {
				mockService.EXPECT().
					GetMetrics(gomock.Any()).
					Return(model.Metrics{}, fmt.Errorf("unknown metrics ID")).
					MinTimes(0)
			} else {
				mockService.EXPECT().GetMetrics(gomock.Any()).Return(testMetrics, nil).Times(1)
			}
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)
			if tt.wantErr {
				assert.Equal(t, tt.errCode, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestHandlerMux_UpdateWithJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)

	tests := []struct {
		err            error
		name           string
		requestBody    string
		expectedBody   string
		nUpdateCalls   int
		expectedStatus int
	}{
		{
			name: "Positive test - counter metrics",
			requestBody: `{
				"id": "cpu",
				"type": "counter",
				"delta": 42
			}`,
			err:            nil,
			nUpdateCalls:   1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Positive test - gauge metrics",
			requestBody: `{
				"id": "memory",
				"type": "gauge",
				"value": 3.14
			}`,
			err:            nil,
			nUpdateCalls:   1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Positive test - new metrics created",
			requestBody: `{
				"id": "new_metric",
				"type": "counter",
				"delta": 1
			}`,
			err:            service.ErrMetricCreated,
			nUpdateCalls:   1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Negative test - invalid JSON",
			requestBody: `{
				"id": "cpu",
				"type": "counter",
				"delta": "not_a_number"
			}`,
			err:            nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Negative test - missing required fields",
			requestBody: `{
				"type": "counter"
			}`,
			err:            nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Negative test - service error",
			requestBody: `{
				"id": "cpu",
				"type": "counter",
				"delta": 65
			}`,
			err:            fmt.Errorf("any error"),
			nUpdateCalls:   1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			if tt.nUpdateCalls > 0 {
				mockService.EXPECT().UpdateMetrics(gomock.Any()).Times(tt.nUpdateCalls).Return(tt.err)
			}
			h := NewHandlerMux(mockService, nil, "", "", "", nil, "")

			r := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBufferString(tt.requestBody))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestHandlerMux_GetMetricsJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)

	counterValue := int64(42)
	gaugeValue := 3.14

	tests := []struct {
		mockSetup      func()
		name           string
		requestBody    string
		expectedBody   string
		expectedStatus int
		checkJSON      bool
	}{
		{
			name: "Positive test - get counter metrics",
			requestBody: `{
				"id": "cpu",
				"type": "counter"
			}`,
			mockSetup: func() {
				mockService.EXPECT().GetMetrics("cpu").
					Return(model.Metrics{
						ID:    "cpu",
						MType: model.Counter,
						Delta: &counterValue,
					}, nil).Times(1)
			},
			expectedStatus: http.StatusOK,
			checkJSON:      true,
		},
		{
			name: "Positive test - get gauge metrics",
			requestBody: `{
				"id": "memory",
				"type": "gauge"
			}`,
			mockSetup: func() {
				mockService.EXPECT().GetMetrics("memory").
					Return(model.Metrics{
						ID:    "memory",
						MType: model.Gauge,
						Value: &gaugeValue,
					}, nil).Times(1)
			},
			expectedStatus: http.StatusOK,
			checkJSON:      true,
		},
		{
			name: "Negative test - metrics not found",
			requestBody: `{
				"id": "nonexistent",
				"type": "counter"
			}`,
			mockSetup: func() {
				mockService.EXPECT().GetMetrics("nonexistent").
					Return(model.Metrics{}, fmt.Errorf("not found")).Times(1)
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "cannot find metrics with ID: nonexistent",
		},
		{
			name: "Negative test - invalid JSON",
			requestBody: `{
				"id": "cpu",
				"type": "counter",
			}`, // trailing comma - invalid JSON
			mockSetup:      func() {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Negative test - unknown metrics type",
			requestBody: `{
				"id": "cpu",
				"type": "invalid_type"
			}`,
			mockSetup:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "unknown metrics type: invalid_type",
		},
		{
			name: "Negative test - missing ID field",
			requestBody: `{
				"type": "counter"
			}`,
			mockSetup:      func() {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			h := NewHandlerMux(mockService, nil, "", "", "", nil, "")

			r := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewBufferString(tt.requestBody))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}

			if tt.checkJSON {
				var response model.Metrics
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.ID)
			}
		})
	}
}

func TestHandlerMux_GetMetricsJSON_ContentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)

	counterValue := int64(100)
	mockService.EXPECT().GetMetrics("test_counter").
		Return(model.Metrics{
			ID:    "test_counter",
			MType: model.Counter,
			Delta: &counterValue,
		}, nil).Times(1)

	h := NewHandlerMux(mockService, nil, "", "", "", nil, "")

	requestBody := `{
		"id": "test_counter",
		"type": "counter"
	}`

	r := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewBufferString(requestBody))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response model.Metrics
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test_counter", response.ID)
	assert.Equal(t, model.Counter, response.MType)
	assert.Equal(t, int64(100), *response.Delta)
}

func TestHandlerMux_UpdateWithJSON_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "Empty request body",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Malformed JSON",
			requestBody:    `{"id": "test", "type": "counter", "delta": }`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlerMux(mockService, nil, "", "", "", nil, "")

			r := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBufferString(tt.requestBody))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
