package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	kafkaio "github.com/segmentio/kafka-go"
)

// Message is the payload written to Kafka and consumed by message-worker.
type Message struct {
	ID             string `json:"id"`
	RoomID         string `json:"room_id"`
	SenderID       string `json:"sender_id"`
	SenderUsername string `json:"sender_username"`
	Content        string `json:"content"`
}

// Producer writes messages to per-room Kafka topics.
type Producer struct {
	writer *kafkaio.Writer
}

func NewProducer(brokers string) *Producer {
	w := &kafkaio.Writer{
		Addr:                   kafkaio.TCP(strings.Split(brokers, ",")...),
		Balancer:               &kafkaio.LeastBytes{},
		RequiredAcks:           kafkaio.RequireAll, // acks=all — NFR-3
		AllowAutoTopicCreation: false,
	}
	return &Producer{writer: w}
}

func (p *Producer) Produce(ctx context.Context, msg Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("kafka.Produce marshal: %w", err)
	}
	return p.writer.WriteMessages(ctx, kafkaio.Message{
		Topic: "room." + msg.RoomID,
		Key:   []byte(msg.ID),
		Value: b,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// CreateTopic provisions topic "room.<roomID>" on the broker.
func CreateTopic(brokerAddr, roomID string) error {
	conn, err := kafkaio.Dial("tcp", brokerAddr)
	if err != nil {
		return fmt.Errorf("kafka.CreateTopic dial: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("kafka.CreateTopic controller: %w", err)
	}

	controllerConn, err := kafkaio.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("kafka.CreateTopic controllerDial: %w", err)
	}
	defer controllerConn.Close()

	return controllerConn.CreateTopics(kafkaio.TopicConfig{
		Topic:             "room." + roomID,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
}
