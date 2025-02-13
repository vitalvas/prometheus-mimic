package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
)

func TestGetKafkaKey(t *testing.T) {
	tests := []struct {
		name   string
		labels []prompb.Label
		want   string
	}{
		{
			name: "metric name present",
			labels: []prompb.Label{
				{Name: "__name__", Value: "metric_name"},
				{Name: "label1", Value: "value1"},
			},
			want: "metric_name",
		},
		{
			name: "metric name absent",
			labels: []prompb.Label{
				{Name: "label1", Value: "value1"},
				{Name: "label2", Value: "value2"},
			},
			want: "h-11036765252144760745",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getKafkaKey(tt.labels)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteHeadersMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		headers        map[string]string
		expectedStatus int
	}{
		{
			name: "valid headers",
			headers: map[string]string{
				"Content-Encoding":                  "snappy",
				"Content-Type":                      "application/x-protobuf",
				"X-Prometheus-Remote-Write-Version": "0.1.0",
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "invalid Content-Encoding",
			headers: map[string]string{
				"Content-Encoding":                  "gzip",
				"Content-Type":                      "application/x-protobuf",
				"X-Prometheus-Remote-Write-Version": "0.1.0",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing Content-Type",
			headers: map[string]string{
				"Content-Encoding":                  "snappy",
				"X-Prometheus-Remote-Write-Version": "0.1.0",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid Content-Type",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing protocol version",
			headers: map[string]string{
				"Content-Encoding": "snappy",
				"Content-Type":     "application/x-protobuf",
			},
			expectedStatus: http.StatusBadRequest,
		},

		{
			name: "missing Content-Encoding in X-Prometheus-Remote-Write-Version",
			headers: map[string]string{
				"Content-Type":                      "application/x-protobuf",
				"X-Prometheus-Remote-Write-Version": "0.1.0",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid Content-Encoding in X-Prometheus-Remote-Write-Version",
			headers: map[string]string{
				"Content-Encoding":                  "gzip",
				"Content-Type":                      "application/x-protobuf",
				"X-Prometheus-Remote-Write-Version": "0.1.0",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid X-Prometheus-Remote-Write-Version",
			headers: map[string]string{
				"Content-Encoding":                  "snappy",
				"Content-Type":                      "application/x-protobuf",
				"X-Prometheus-Remote-Write-Version": "0.2.0",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "valid headers with X-Prometheus-Remote-Write-Version",
			headers: map[string]string{
				"Content-Encoding":                  "snappy",
				"Content-Type":                      "application/x-protobuf",
				"X-Prometheus-Remote-Write-Version": "0.1.0",
			},
			expectedStatus: http.StatusNoContent,
		},

		{
			name: "missing Content-Encoding in X-VictoriaMetrics-Remote-Write-Version",
			headers: map[string]string{
				"Content-Type":                           "application/x-protobuf",
				"X-VictoriaMetrics-Remote-Write-Version": "1",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid Content-Encoding in X-VictoriaMetrics-Remote-Write-Version",
			headers: map[string]string{
				"Content-Encoding":                       "gzip",
				"Content-Type":                           "application/x-protobuf",
				"X-VictoriaMetrics-Remote-Write-Version": "1",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid X-Prometheus-Remote-Write-Version",
			headers: map[string]string{
				"Content-Encoding":                       "zstd",
				"Content-Type":                           "application/x-protobuf",
				"X-VictoriaMetrics-Remote-Write-Version": "777",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "valid headers with X-VictoriaMetrics-Remote-Write-Version",
			headers: map[string]string{
				"Content-Encoding":                       "zstd",
				"Content-Type":                           "application/x-protobuf",
				"X-VictoriaMetrics-Remote-Write-Version": "1",
			},
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(writeHeadersMiddleware)
			r.POST("/", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer([]byte{}))
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}

	t.Run("vicotria metrics protocol test", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(writeHeadersMiddleware)
		r.POST("/", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodPost, "/?get_vm_proto_version=1", bytes.NewBuffer([]byte{}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		assert.Equal(t, "1", w.Body.String())
	})

	t.Run("vicotria metrics protocol test fail", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(writeHeadersMiddleware)
		r.POST("/", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodPost, "/?get_vm_proto_version=22", bytes.NewBuffer([]byte{}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
