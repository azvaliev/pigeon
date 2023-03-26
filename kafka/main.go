package kafkahelpers;

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"

	"github.com/oklog/ulid/v2"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// Represents the subset of the message record stored in kafka
type Message struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Message string `json:"message"`
}

func GetKafkaDialer() *kafka.Dialer {
	dialer := &kafka.Dialer{}

	// Use credentials if available. Locally not needed
	kafkaUsername, kafkaPassword := os.Getenv("KAFKA_USERNAME"), os.Getenv("KAFKA_PASSWORD")
	if kafkaUsername != "" && kafkaPassword != "" {
		mechanism, err := scram.Mechanism(scram.SHA256, kafkaUsername, kafkaPassword)
		if err != nil {
			return nil
		}

		dialer.SASLMechanism = mechanism
		dialer.TLS = &tls.Config{}
	}

	return dialer
}

func KafkaLogger(msg string, a ...interface{}) {
	fmt.Printf(msg, a...)
	fmt.Println()
}

// Uses ULIDPartioner, will panic on error as kafka.BalancerFunc can only return an int
var ULIDBalancer = kafka.BalancerFunc(func(msg kafka.Message, partions ...int) int {
	var message Message
	err := json.Unmarshal(msg.Value, &message)

	if err != nil {
		panic(err)
	}

	partition, err := ULIDPartioner(message.To, len(partions))

	if err != nil {
		panic(err)
	}

	return *partition
})

// Since a ULID encodes a timestamp in its first 48 bits,
// we can use that to determine which partition to send the message to
func ULIDPartioner(recipientId string, numPartions int) (*int, error) {
	recipientUlid, err := ulid.Parse(recipientId)

	if err != nil {
		return nil, err
	}

	recipientUlidTimestamp := int(recipientUlid.Time())
  recipientPartition := recipientUlidTimestamp % numPartions

	return &recipientPartition, nil
}
