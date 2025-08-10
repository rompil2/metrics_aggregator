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
	"github.com/stretchr/testify/require"
)

func TestBuildMetrics(t *testing.T) {
	type args struct {
		components []string
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
			args{[]string{"counter", "cpu", "0"}},
			model.Metrics{
				MType: model.Counter,
				ID:    "cpu",
				Delta: new(int64),
			},
			false,
		}, {
			"Positive Test. Gauge.",
			args{[]string{"gauge", "memory", "0.0"}},
			model.Metrics{
				MType: model.Gauge,
				ID:    "memory",
				Value: new(float64),
			},
			false,
		}, {
			"Negative Test. Counter. Wrong data format.",
			args{[]string{"counter", "cpu", "C"}},
			model.Metrics{},
			true,
		}, {
			"Negative Test. Gauge. Wrong data format.",
			args{[]string{"gauge", "memory", "G"}},
			model.Metrics{},
			true,
		}, {
			"Negative Test. Unknown type.",
			args{[]string{"gouge", "memory", "G"}},
			model.Metrics{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildMetrics(tt.args.components)
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

func TestPathToParse(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			"Positive Test",
			args{"/counter/cpu/1"},
			[]string{"counter", "cpu", "1"},
			false,
		}, {
			"Positive Test, slash at the end",
			args{"/counter/cpu/1/"},
			[]string{"counter", "cpu", "1"},
			false,
		}, {
			"Negative Test, not enough params",
			args{"/counter/cpu/"},
			nil,
			true,
		}, {
			"Negative Test, to many params",
			args{"/counter/cpu/3/test"},
			nil,
			true,
		}, {
			"Negative Test, empty path",
			args{""},
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PathToParse(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("PathToParse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr { // do not expect an error
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("PathToParse() = %v, want %v", got, tt.want)
				}
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
			"Positive test.", "/counter/cpu/1", false, http.StatusOK,
		}, {
			"Negative test. Not enought arguments.", "/counter/cpu/", true, http.StatusBadRequest,
		}, {
			"Negative test.  Wrong metrics type.", "/caunter/cpu/1", true, http.StatusBadRequest,
		}, {
			"Negative test. Wrong data format", "/counter/cpu/t", true, http.StatusBadRequest,
		}, {
			"Negative test. Without counter ID", "/counter/", true, http.StatusNotFound,
		}, {
			"Negative test. Without gauge ID", "/gauge/", true, http.StatusNotFound,
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := HandlerMux{}
			h.Service = mockService
			if !tt.wantErr {
				mockService.EXPECT().UpdateMetrics(gomock.Any()).Return(fmt.Errorf("Unknown metrics ID, created the new one")).Times(1)
			}
			r := httptest.NewRequest(http.MethodPost, tt.path, nil)
			w := httptest.NewRecorder()
			h.UpdateMetrics(w, r)
			if tt.wantErr {
				assert.Equal(t, tt.errCode, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestMiddlewarePostOnly(t *testing.T) {
	tests := []struct {
		Name     string
		Method   string
		wantCode int
		wantBody string
	}{
		{"Valid_POST_Request", http.MethodPost, http.StatusOK, ""},
		{"Invalid_GET_Request", http.MethodGet, http.StatusMethodNotAllowed, "Only POST method is allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tt.Method, "/", nil)

			dummyHandler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {})

			MiddlewarePostOnly(dummyHandler)(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			require.Equal(t, tt.wantCode, resp.StatusCode)

			if tt.wantBody != "" {
				assert.Contains(t, recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestMiddlewareGetOnly(t *testing.T) {
	tests := []struct {
		Name     string
		Method   string
		wantCode int
		wantBody string
	}{
		{"Valid_Get_Request", http.MethodGet, http.StatusOK, ""},
		{"Invalid_POST_Request", http.MethodPost, http.StatusMethodNotAllowed, "Only Get method is allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tt.Method, "/", nil)

			dummyHandler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {})

			MiddlewareGetOnly(dummyHandler)(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			require.Equal(t, tt.wantCode, resp.StatusCode)

			if tt.wantBody != "" {
				assert.Contains(t, recorder.Body.String(), tt.wantBody)
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
			"Positive test.", "/counter/cpu", false, http.StatusOK,
		}, {
			"Negative test. Without counter ID.", "/counter", true, http.StatusBadRequest,
		}, {
			"Negative test. Unknow ID", "/counter/memory", true, http.StatusNotFound,
		}, {
			"Negative test. Empty path", "/", true, http.StatusBadRequest,
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockService := mocks.NewMockService(ctrl)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := HandlerMux{}
			h.Service = mockService
			if tt.wantErr {
				mockService.EXPECT().GetMetrics(gomock.Any()).Return(model.Metrics{}, fmt.Errorf("unknown metrics ID")).MinTimes(0)
			} else {
				mockService.EXPECT().GetMetrics(gomock.Any()).Return(testMetrics, nil).Times(1)
			}
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			h.GetMetrics(w, r)
			if tt.wantErr {
				assert.Equal(t, tt.errCode, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}
