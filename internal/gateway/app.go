package gateway

import (
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

type Gateway struct {
	config        *Config
	kafkaClient   sarama.Client
	kafkaProducer sarama.AsyncProducer

	lastErrorTime time.Time
}

func New(configPath string) (*Gateway, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	kafkaProducer, kafkaClient, err := newKafka(config)
	if err != nil {
		log.Fatalf("failed to create kafka producer: %v", err)
	}

	gateway := &Gateway{
		config:        config,
		kafkaClient:   kafkaClient,
		kafkaProducer: kafkaProducer,
	}

	go gateway.monitorKafkaHealth()

	return gateway, nil
}

func newKafka(config *Config) (sarama.AsyncProducer, sarama.Client, error) {
	kafkaConf := sarama.NewConfig()
	kafkaConf.ClientID = "mimic-gateway"
	kafkaConf.Version = sarama.V2_8_0_0
	kafkaConf.Producer.RequiredAcks = sarama.WaitForLocal
	kafkaConf.Producer.Flush.Frequency = 10 * time.Second
	kafkaConf.Producer.Flush.Bytes = maxInsertRequestSize / 4
	kafkaConf.Producer.Flush.Messages = 100_000

	client, err := sarama.NewClient(config.Kafka.Brokers, kafkaConf)
	if err != nil {
		return nil, nil, err
	}

	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		return nil, nil, err
	}

	return producer, client, nil
}

func (g *Gateway) monitorKafkaHealth() {
	for err := range g.kafkaProducer.Errors() {
		log.Printf("failed to write entry: %s", err.Error())

		g.lastErrorTime = time.Now()
	}
}
