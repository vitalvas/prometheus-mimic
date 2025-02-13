package gateway

import (
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/prometheus/prompb"
)

func getKafkaKey(labels []prompb.Label) string {
	for _, label := range labels {
		//  __name__ is a special label that contains the metric name
		if label.Name == "__name__" {
			return label.Value
		}
	}

	hash := fnv.New64a()
	for _, label := range labels {
		hash.Write([]byte(label.Name))
		hash.Write([]byte(label.Value))
	}

	return fmt.Sprintf("h-%d", hash.Sum64())
}

func writeHeadersMiddleware(c *gin.Context) {
	if value, ok := c.GetQuery("get_vm_proto_version"); ok && value == "1" {
		c.String(http.StatusOK, "1")
		c.Abort()
		return
	}

	if c.GetHeader("Content-Type") != "application/x-protobuf" {
		c.String(http.StatusBadRequest, "unsupported Content-Type: %s", c.GetHeader("Content-Type"))
		c.Abort()
		return
	}

	writeProtocol := "none"

	// Prometheus Remote Write protocol
	if value := c.GetHeader("X-Prometheus-Remote-Write-Version"); value != "" {
		if value != "0.1.0" {
			c.String(http.StatusBadRequest, "unsupported X-Prometheus-Remote-Write-Version: %s", value)
			c.Abort()
			return
		}

		if c.GetHeader("Content-Encoding") != "snappy" {
			c.String(http.StatusBadRequest, "unsupported Content-Encoding for Prometheus: %s", c.GetHeader("Content-Encoding"))
			c.Abort()
			return
		}

		writeProtocol = "prometheus"
	}

	// VictoriaMetrics remote write protocol
	if value := c.GetHeader("X-VictoriaMetrics-Remote-Write-Version"); value != "" {
		if value != "1" {
			c.String(http.StatusBadRequest, "unsupported X-VictoriaMetrics-Remote-Write-Version: %s", value)
			c.Abort()
			return
		}

		if c.GetHeader("Content-Encoding") != "zstd" {
			c.String(http.StatusBadRequest, "unsupported Content-Encoding for VictoriaMetrics: %s", c.GetHeader("Content-Encoding"))
			c.Abort()
			return
		}

		writeProtocol = "victoriametrics"
	}

	if writeProtocol == "none" {
		c.String(http.StatusBadRequest, "unsupported remote write protocol")
		c.Abort()
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxInsertRequestSize)

	c.Next()
}

func (g *Gateway) writeHandler(c *gin.Context) {
	if !g.lastErrorTime.IsZero() && g.lastErrorTime.Unix() >= time.Now().Add(-10*time.Second).Unix() {
		c.String(http.StatusServiceUnavailable, "gateway is in error state")
		return
	}

	started := time.Now()

	metricWriteBatchesRequests.Inc()

	authenticatedUser := c.MustGet("user").(*User)

	compressed, err := io.ReadAll(c.Request.Body)
	if err != nil {
		if strings.Contains(err.Error(), "request too large") {
			c.String(http.StatusRequestEntityTooLarge, "request body is too large")
		} else {
			c.String(http.StatusInternalServerError, "error reading request body: %v", err)
		}
		return
	}

	metricWriteBatchesReceivedBytes.Add(float64(len(compressed)))

	var requestBuffer []byte

	switch c.GetHeader("Content-Encoding") {
	case "snappy":
		requestBuffer, err = decompressSnappy(compressed)
		if err != nil {
			c.String(http.StatusBadRequest, "error decoding snappy: %v", err)
			return
		}

		metricWriteBatchesRequestsEncoding.WithLabelValues("snappy").Inc()

	case "zstd":
		requestBuffer, err = decompressZSTD(compressed)
		if err != nil {
			c.String(http.StatusBadRequest, "error decoding zstd: %v", err)
			return
		}

		metricWriteBatchesRequestsEncoding.WithLabelValues("zstd").Inc()

	default:
		c.String(http.StatusBadRequest, "unsupported Content-Encoding: %s", c.GetHeader("Content-Encoding"))
		return
	}

	metricWriteBatchesReceivedUncompressedBytes.Add(float64(len(requestBuffer)))

	var req prompb.WriteRequest
	if err := proto.Unmarshal(requestBuffer, &req); err != nil {
		c.String(http.StatusBadRequest, "error unmarshaling protobuf: %v", err)
		return
	}

	kafkaTopic := g.config.Kafka.Topic
	if authenticatedUser.Topic != nil {
		kafkaTopic = *authenticatedUser.Topic
	}

	for _, ts := range req.GetTimeseries() {
		// reconstruct the original TimeSeries
		messgaeWriteRequest := &prompb.TimeSeries{
			Labels:     ts.Labels,
			Exemplars:  ts.Exemplars,
			Samples:    ts.Samples,
			Histograms: ts.Histograms,
		}

		messageBytes, err := proto.Marshal(messgaeWriteRequest)
		if err != nil {
			c.String(http.StatusInternalServerError, "error marshaling protobuf: %v", err)
			return
		}

		message := &sarama.ProducerMessage{
			Topic: kafkaTopic,
			Key:   sarama.StringEncoder(getKafkaKey(ts.Labels)),
			Value: sarama.ByteEncoder(messageBytes),
		}

		metricWriteKafkaMessages.WithLabelValues(kafkaTopic).Inc()

		select {
		case g.kafkaProducer.Input() <- message:
			continue

		case <-time.After(g.getKafkaWriteTimeout()):
			c.String(http.StatusServiceUnavailable, "timeout writing to kafka")
		}
	}

	metricsWriteBatchesRequestsDuration.Observe(time.Since(started).Seconds())

	c.Status(http.StatusNoContent)
}

func (g *Gateway) getKafkaWriteTimeout() time.Duration {
	config := g.kafkaClient.Config()

	return config.Producer.Timeout * time.Duration(config.Producer.Retry.Max)
}
