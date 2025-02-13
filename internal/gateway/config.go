package gateway

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kafka KafkaConfig `yaml:"kafka"`
	Users []User      `yaml:"users"`
}

type KafkaConfig struct {
	Topic   string   `yaml:"topic"`
	Brokers []string `yaml:"brokers"`
}

type User struct {
	Login    string  `yaml:"login"`
	Password string  `yaml:"password"`
	Topic    *string `yaml:"topic"`
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &Config{}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml config: %w", err)
	}

	return config, nil
}
