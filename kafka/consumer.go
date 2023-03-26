package kafkahelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/segmentio/kafka-go"
)

// Handler function for processing messages. Return true to continue processing, false to stop processing
type ProcessMessageHandler func(message *Message) bool

// Decodes messages for a specific recipient and executes callback function
// Return true from handler function to continue processing, false to stop processing
func ProcessMessages(recipientId string, reader *kafka.Reader, messageHandler ProcessMessageHandler) error {
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("Failed to read message: %v\n", err)
			return err
		}

		// Use the key to determine if the message is for the recipient
		if string(m.Key) != recipientId {
			continue
		}

		// Turn m into Message struct
		message := &Message{}
		err = json.Unmarshal(m.Value, message)
		if err != nil {
			return err
		}

		ok := messageHandler(message)
		if ok == false {
			break
		}
	}

	if err := reader.Close(); err != nil {
		fmt.Printf("Failed to close reader: %v\n", err)
		return err
	}

	return nil
}

// Create an Kafka consumer for a specific recipient
func CreateConsumer(recipientId string) (*kafka.Reader, error) {
	kafkaBroker, kafkaTopic := os.Getenv("KAFKA_BROKER"), os.Getenv("KAFKA_TOPIC")

	// Determine the current number of partitions
	dialer := GetKafkaDialer()
	conn, err := dialer.DialLeader(context.Background(), "tcp", kafkaBroker, kafkaTopic, 0)
	if err != nil {
		fmt.Printf("Failed to connect to Kafka leader: %v\n", err)
		return nil, err
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(kafkaTopic)
	if err != nil {
		fmt.Printf("Failed to read partitions: %v\n", err)
		return nil, err
	}

	// Use total partition number to determine which partition to read from
	consumerPartition, err := ULIDPartioner(recipientId, len(partitions))

	if err != nil {
		fmt.Printf("Failed to determine partition for %s: %v\n", recipientId, err)
		return nil, err
	}

	// Create a new reader i.e. consumer
	consumer := kafka.NewReader(kafka.ReaderConfig{
		Brokers:               []string{kafkaBroker},
		Topic:                 kafkaTopic,
		Partition:             *consumerPartition,
		Dialer:                GetKafkaDialer(),
		WatchPartitionChanges: true,
		StartOffset:           kafka.LastOffset,
		Logger:                kafka.LoggerFunc(KafkaLogger),
		ErrorLogger:           kafka.LoggerFunc(KafkaLogger),
		IsolationLevel:        0,
	})

	err = consumer.SetOffset(kafka.LastOffset)
	if err != nil {
		fmt.Printf("Failed to set offset for %s: %v\n", recipientId, err)
		return nil, err
	}

	return consumer, nil
}
