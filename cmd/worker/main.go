package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/prompb"
)

func main() {
	kafkaConf := sarama.NewConfig()
	kafkaConf.Version = sarama.V2_8_0_0
	kafkaConf.Consumer.Return.Errors = true
	kafkaConf.Consumer.Offsets.Initial = sarama.OffsetOldest

	topics := []string{"vmcluster_default"}
	if row, ok := os.LookupEnv("MIMIC_KAFKA_TOPICS"); ok {
		topics = strings.Split(row, ",")
	}

	brokers := []string{"kafka:9092"}
	if row, ok := os.LookupEnv("MIMIC_KAFKA_BROKERS"); ok {
		brokers = strings.Split(row, ",")
	}

	groupID := "prometheus-mimic-worker"
	if row, ok := os.LookupEnv("MIMIC_KAFKA_GROUP_ID"); ok {
		groupID = row
	}

	client, err := sarama.NewConsumerGroup(brokers, groupID, kafkaConf)
	if err != nil {
		log.Fatalf("error creating consumer group: %v", err)
	}

	defer client.Close()

	if endpoint, ok := os.LookupEnv("MIMIC_METRICS_LISTEN"); ok {
		go func() {
			router := http.NewServeMux()
			router.Handle("/metrics", promhttp.Handler())

			router.HandleFunc("/debug/pprof/", http.HandlerFunc(pprof.Index))
			router.HandleFunc("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
			router.HandleFunc("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
			router.HandleFunc("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
			router.HandleFunc("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

			if err := http.ListenAndServe(endpoint, router); err != nil {
				log.Fatalf("error starting metrics server: %v", err)
			}
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	writeEndpoint := "http://victoriametrics:8428/api/v1/write"
	if row, ok := os.LookupEnv("MIMIC_WRITE_ENDPOINT"); ok {
		writeEndpoint = row
	}

	consumer := NewConsumer(writeEndpoint)

	go func() {
		defer wg.Done()

		for {
			if err := client.Consume(ctx, topics, consumer); err != nil {
				log.Printf("error consuming: %v", err)
			}

			if ctx.Err() != nil {
				return
			}

			consumer.ready = make(chan bool)
		}
	}()
	<-consumer.ready

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	<-sigterm

	cancel()
	wg.Wait()
}

func NewConsumer(writeEndpoint string) *Consumer {
	return &Consumer{
		ready: make(chan bool),

		remoteURL: writeEndpoint,

		batchLen:  100_000,
		batchSize: 30 * 1024 * 1024, // 30MB, no more than maxInsertRequestSize (victoria-metrics)
		batchTime: time.Second,

		httpClient: &http.Client{},
	}
}

type Consumer struct {
	ready     chan bool
	remoteURL string

	batchLen  int
	batchSize int
	batchTime time.Duration

	httpClient *http.Client
}

func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	close(consumer.ready)
	return nil
}

func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	messages := make([]*sarama.ConsumerMessage, 0, consumer.batchSize)
	var messagesSize int

	batchTicker := time.NewTicker(consumer.batchTime)
	defer batchTicker.Stop()

	defer func() {
		if len(messages) > 0 {
			consumer.processMessages(messages)
			messages = nil
		}
	}()

	for {
		select {
		case msg := <-claim.Messages():
			if msg == nil {
				continue // ignore nil messages
			}

			messagesSize += len(msg.Value)

			messages = append(messages, msg)
			session.MarkMessage(msg, "")

			if len(messages) >= consumer.batchLen || messagesSize >= consumer.batchSize {
				consumer.processMessages(messages)
				messages = nil
				messagesSize = 0

				batchTicker.Reset(consumer.batchTime)
			}

		case <-batchTicker.C:
			if len(messages) > 0 {
				consumer.processMessages(messages)
				messages = nil
				messagesSize = 0
			}
		}
	}
}

func (consumer *Consumer) processMessages(messages []*sarama.ConsumerMessage) {
	timeSeries := &prompb.WriteRequest{}

	for _, msg := range messages {
		var ts prompb.TimeSeries
		if err := proto.Unmarshal(msg.Value, &ts); err != nil {
			log.Printf("error unmarshaling protobuf: %v", err)
			return
		}

		timeSeries.Timeseries = append(timeSeries.Timeseries, ts)
	}

	messageBytes, err := proto.Marshal(timeSeries)
	if err != nil {
		log.Printf("error marshaling protobuf: %v", err)
		return
	}

	messageBytesCompressed := snappy.Encode(nil, messageBytes)

	for i := 0; i < 1024; i++ {
		if err := consumer.sendMessages(messageBytesCompressed); err != nil {
			log.Printf("error sending messages: %v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
}

func (consumer *Consumer) sendMessages(payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, consumer.remoteURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := consumer.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}

	defer resp.Body.Close()

	if !slices.Contains([]int{http.StatusOK, http.StatusNoContent}, resp.StatusCode) {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send batch. Unexpected status code: %d. Body: %s", resp.StatusCode, string(body))
	}

	return nil
}
