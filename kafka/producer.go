package kafkahelpers

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

func PostMessage(producer *kafka.Writer, message *Message) error {
	encodedMessage, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Create a new message to write to kafka
	kafkaMessage := kafka.Message{
		Key: []byte(message.To),
		Value: []byte(
			// convert struct into JSON binary
			encodedMessage,
		),
	}

	// Write messages to topic (asynchronously)
	err = producer.WriteMessages(context.Background(), kafkaMessage)
	if err != nil {
		return err
	}

	return nil
}

// Create an Kafka producer for a specific conversation
func CreateProducer(conversationId string) *kafka.Writer {
	kafkaBroker, kafkaTopic := os.Getenv("KAFKA_BROKER"), os.Getenv("KAFKA_TOPIC")

	dialer := GetKafkaDialer()

	// Create a new writer
	return &kafka.Writer{
		Addr:                   kafka.TCP(kafkaBroker),
		Topic:                  kafkaTopic,
		Balancer:               ULIDBalancer,
		MaxAttempts:            10,
		WriteTimeout:           10 * time.Second,
		Async:                  true,
		AllowAutoTopicCreation: true,
		Logger:                 kafka.LoggerFunc(KafkaLogger),
		ErrorLogger:            kafka.LoggerFunc(KafkaLogger),
		Compression:            kafka.Snappy,
		Transport: &kafka.Transport{
			TLS:  dialer.TLS,
			SASL: dialer.SASLMechanism,
		},
	}
}
