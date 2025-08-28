//nolint:bodyclose
package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rompil2/metrics_aggregator/internal/mocks"
	"github.com/rompil2/metrics_aggregator/internal/model"
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
			h := NewHandlerMux(mockService, nil)
			if !tt.wantErr {
				mockService.EXPECT().UpdateMetrics(gomock.Any()).Return(fmt.Errorf("Unknown metrics ID, created the new one")).Times(1)
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
		}, {
			"Negative test. Empty path", "/value/", true, http.StatusNotFound,
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlerMux(mockService, nil)
			if tt.wantErr {
				mockService.EXPECT().GetMetrics(gomock.Any()).Return(model.Metrics{}, fmt.Errorf("unknown metrics ID")).MinTimes(0)
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
