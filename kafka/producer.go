package kafka

import (
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

// Create an Kafka producer for a specific recipient
func CreateProducer(recipientId string) *kafka.Writer {
	kafkaBroker, kafkaTopic := os.Getenv("KAFKA_BROKER"), os.Getenv("KAFKA_TOPIC")

  dialer := GetKafkaDialer();

	// Create a new writer
	return &kafka.Writer{
		Addr:                   kafka.TCP(kafkaBroker),
		Topic:                  kafkaTopic,
		Balancer:               ULIDBalancer,
		MaxAttempts:            10,
		WriteTimeout:           10 * time.Second,
		Async:                  false,
		AllowAutoTopicCreation: true,
		Logger:                 kafka.LoggerFunc(KafkaLogger),
		ErrorLogger:            kafka.LoggerFunc(KafkaLogger),
		Compression:            kafka.Snappy,
    Transport: &kafka.Transport{
    	TLS:         dialer.TLS,
    	SASL:        dialer.SASLMechanism,
    },
	}
}
