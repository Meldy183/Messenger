package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	kafkaio "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaMessage is the payload produced by chat-service.
type KafkaMessage struct {
	ID             string `json:"id"`
	RoomID         string `json:"room_id"`
	SenderID       string `json:"sender_id"`
	SenderUsername string `json:"sender_username"`
	Content        string `json:"content"`
}

type broadcastPayload struct {
	RoomID  string          `json:"room_id"`
	Message messageResponse `json:"message"`
}

type messageResponse struct {
	ID             string    `json:"id"`
	RoomID         string    `json:"room_id"`
	SenderID       string    `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// Worker polls Kafka topics for all rooms and dispatches messages.
// It periodically refreshes its topic list from Postgres so that new rooms
// are picked up without a restart (refresh interval = 30s).
type Worker struct {
	brokers        []string
	groupID        string
	db             *sql.DB
	chatServiceURL string
	log            *zap.Logger
}

func New(brokers, groupID string, db *sql.DB, chatServiceURL string, log *zap.Logger) *Worker {
	return &Worker{
		brokers:        strings.Split(brokers, ","),
		groupID:        groupID,
		db:             db,
		chatServiceURL: chatServiceURL,
		log:            log,
	}
}

// Run starts the consume loop, refreshing the topic list every 30 seconds.
func (w *Worker) Run(ctx context.Context) error {
	var (
		reader  *kafkaio.Reader
		current []string
	)
	defer func() {
		if reader != nil {
			reader.Close()
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		topics, err := w.fetchTopics(ctx)
		if err != nil {
			w.log.Warn("failed to fetch room topics", zap.Error(err))
		} else if !equal(current, topics) {
			if reader != nil {
				reader.Close()
			}
			if len(topics) > 0 {
				reader = w.newReader(topics)
				w.log.Info("subscribed to topics", zap.Strings("topics", topics))
			} else {
				reader = nil
				w.log.Info("no topics to subscribe to, waiting")
			}
			current = topics
		}

		if reader != nil {
			if err := w.consumeBatch(ctx, reader, ticker); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		} else {
			// No rooms yet — wait for the next tick.
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
	}
}

// consumeBatch runs the read loop until either a ticker fires (to refresh topics)
// or the context is cancelled.
func (w *Worker) consumeBatch(ctx context.Context, reader *kafkaio.Reader, ticker *time.Ticker) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			return nil // trigger topic refresh
		default:
		}

		fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		m, err := reader.FetchMessage(fetchCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil || fetchCtx.Err() != nil {
				return nil // timeout or shutdown — loop back to check ticker
			}
			w.log.Warn("kafka fetch error", zap.Error(err))
			return nil
		}

		var msg KafkaMessage
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			w.log.Warn("invalid kafka message, skipping", zap.ByteString("raw", m.Value), zap.Error(err))
			_ = reader.CommitMessages(ctx, m)
			continue
		}

		createdAt, inserted, err := w.persist(ctx, msg)
		if err != nil {
			w.log.Error("persist failed, not committing", zap.String("msg_id", msg.ID), zap.Error(err))
			continue // don't commit — Kafka redelivers
		}

		if inserted {
			if err := w.broadcast(msg, createdAt); err != nil {
				w.log.Warn("broadcast failed (message persisted)", zap.String("msg_id", msg.ID), zap.Error(err))
			}
		}

		if err := reader.CommitMessages(ctx, m); err != nil {
			w.log.Warn("kafka commit failed", zap.Error(err))
		}
	}
}

func (w *Worker) newReader(topics []string) *kafkaio.Reader {
	return kafkaio.NewReader(kafkaio.ReaderConfig{
		Brokers:        w.brokers,
		GroupID:        w.groupID,
		GroupTopics:    topics,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0, // manual commit
	})
}

func (w *Worker) fetchTopics(ctx context.Context) ([]string, error) {
	rows, err := w.db.QueryContext(ctx, `SELECT id FROM rooms`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		topics = append(topics, "room."+id)
	}
	return topics, rows.Err()
}

// persist inserts the message with UUID dedup; returns (createdAt, inserted, error).
func (w *Worker) persist(ctx context.Context, msg KafkaMessage) (time.Time, bool, error) {
	var createdAt time.Time
	err := w.db.QueryRowContext(ctx,
		`INSERT INTO messages (id, room_id, sender_id, sender_username, content, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (id) DO NOTHING
		 RETURNING created_at`,
		msg.ID, msg.RoomID, msg.SenderID, msg.SenderUsername, msg.Content,
	).Scan(&createdAt)

	if err == sql.ErrNoRows {
		return time.Time{}, false, nil // duplicate
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return createdAt, true, nil
}

func (w *Worker) broadcast(msg KafkaMessage, createdAt time.Time) error {
	payload := broadcastPayload{
		RoomID: msg.RoomID,
		Message: messageResponse{
			ID:             msg.ID,
			RoomID:         msg.RoomID,
			SenderID:       msg.SenderID,
			SenderUsername: msg.SenderUsername,
			Content:        msg.Content,
			CreatedAt:      createdAt,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(w.chatServiceURL+"/internal/broadcast", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("broadcast returned %d", resp.StatusCode)
	}
	return nil
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, v := range a {
		seen[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := seen[v]; !ok {
			return false
		}
	}
	return true
}
