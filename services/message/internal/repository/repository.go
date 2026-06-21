package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ContentTypeText = "text"

	AckTypeDelivered = "delivered"
	AckTypeRead      = "read"

	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"

	CodeInvalidArgument = "INVALID_ARGUMENT"
	CodeInternal        = "INTERNAL_ERROR"
)

var ErrInvalidArgument = errors.New("invalid argument")

// Message is the persisted message fact returned by Message service.
// Message 是 Message 服务返回的持久化消息事实。
type Message struct {
	MessageID      string
	ConversationID string
	Seq            int64
	SenderID       string
	DeviceID       string
	ClientMsgID    string
	ClientSeq      int64
	ContentType    string
	ContentJSON    []byte
	CreatedAt      time.Time
}

// OutboxEvent is one pending domain event to publish.
// OutboxEvent 是一条待发布领域事件。
type OutboxEvent struct {
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	RoutingKey    string
	PayloadJSON   []byte
	Attempts      int
}

type SaveMessageInput struct {
	ConversationID string
	Seq            int64
	SenderID       string
	DeviceID       string
	ClientMsgID    string
	ClientSeq      int64
	ContentType    string
	ContentJSON    []byte
	OutboxPayload  []byte
	RoutingKey     string
}

// Repository owns message facts, receipts, and outbox persistence.
// Repository 管理消息事实、回执和 outbox 持久化。
type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByIdempotency(ctx context.Context, senderID, deviceID, clientMsgID string) (Message, bool, error) {
	if r == nil || r.db == nil {
		return Message{}, false, errors.New("message repository is not initialized")
	}
	senderID = normalizeID(senderID)
	deviceID = normalizeID(deviceID)
	clientMsgID = normalizeID(clientMsgID)
	if senderID == "" || deviceID == "" || clientMsgID == "" {
		return Message{}, false, fmt.Errorf("%w: sender_id, device_id and client_msg_id are required", ErrInvalidArgument)
	}
	msg, err := scanMessage(r.db.QueryRow(ctx, `
SELECT message_id, conversation_id, seq, sender_id, device_id, client_msg_id, client_seq, content_type, content_json, created_at
FROM messages
WHERE sender_id = $1 AND device_id = $2 AND client_msg_id = $3`, senderID, deviceID, clientMsgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, nil
	}
	if err != nil {
		return Message{}, false, fmt.Errorf("query idempotent message: %w", err)
	}
	return msg, true, nil
}

func (r *Repository) SaveMessageWithOutbox(ctx context.Context, input SaveMessageInput) (Message, bool, error) {
	if r == nil || r.db == nil {
		return Message{}, false, errors.New("message repository is not initialized")
	}
	if err := validateSaveInput(input); err != nil {
		return Message{}, false, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Message{}, false, fmt.Errorf("begin message transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	messageID := uuid.NewString()
	eventID := uuid.NewString()
	msg, err := scanMessage(tx.QueryRow(ctx, `
INSERT INTO messages (
    message_id, conversation_id, seq, sender_id, device_id, client_msg_id, client_seq, content_type, content_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
ON CONFLICT (sender_id, device_id, client_msg_id) DO NOTHING
RETURNING message_id, conversation_id, seq, sender_id, device_id, client_msg_id, client_seq, content_type, content_json, created_at`,
		messageID,
		input.ConversationID,
		input.Seq,
		input.SenderID,
		input.DeviceID,
		input.ClientMsgID,
		input.ClientSeq,
		input.ContentType,
		string(input.ContentJSON),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		existing, ok, findErr := r.FindByIdempotency(ctx, input.SenderID, input.DeviceID, input.ClientMsgID)
		if findErr != nil {
			return Message{}, false, findErr
		}
		if !ok {
			return Message{}, false, errors.New("idempotent message conflict was not visible after insert")
		}
		return existing, true, nil
	}
	if err != nil {
		return Message{}, false, fmt.Errorf("insert message: %w", err)
	}

	outboxPayload, err := enrichOutboxPayload(input.OutboxPayload, eventID, msg)
	if err != nil {
		return Message{}, false, err
	}

	if _, err = tx.Exec(ctx, `
INSERT INTO outbox_events (
    event_id, aggregate_type, aggregate_id, event_type, routing_key, payload_json, status
)
VALUES ($1, 'message', $2, 'message.created', $3, $4::jsonb, 'pending')`,
		eventID,
		msg.MessageID,
		input.RoutingKey,
		string(outboxPayload),
	); err != nil {
		return Message{}, false, fmt.Errorf("insert message outbox event: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Message{}, false, fmt.Errorf("commit message transaction: %w", err)
	}
	return msg, false, nil
}

func (r *Repository) AckMessages(ctx context.Context, conversationID, userID, ackType string, seqs []int64) (int32, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("message repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	userID = normalizeID(userID)
	ackType = strings.ToLower(strings.TrimSpace(ackType))
	seqs = normalizeSeqs(seqs)
	if conversationID == "" || userID == "" || len(seqs) == 0 {
		return 0, fmt.Errorf("%w: conversation_id, user_id and seqs are required", ErrInvalidArgument)
	}
	if ackType != AckTypeDelivered && ackType != AckTypeRead {
		return 0, fmt.Errorf("%w: unsupported ack_type", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin ack transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var updated int64
	for _, seq := range seqs {
		var tag pgconnTag
		switch ackType {
		case AckTypeDelivered:
			tag, err = tx.Exec(ctx, `
INSERT INTO message_receipts (conversation_id, user_id, message_seq, delivered_at)
SELECT $1::varchar, $2::varchar, m.seq, NOW()
FROM messages m
WHERE m.conversation_id = $1::varchar AND m.seq = $3::bigint
ON CONFLICT (conversation_id, user_id, message_seq)
DO UPDATE SET delivered_at = COALESCE(message_receipts.delivered_at, EXCLUDED.delivered_at)`, conversationID, userID, seq)
		case AckTypeRead:
			tag, err = tx.Exec(ctx, `
INSERT INTO message_receipts (conversation_id, user_id, message_seq, delivered_at, read_at)
SELECT $1::varchar, $2::varchar, m.seq, NOW(), NOW()
FROM messages m
WHERE m.conversation_id = $1::varchar AND m.seq = $3::bigint
ON CONFLICT (conversation_id, user_id, message_seq)
DO UPDATE SET delivered_at = COALESCE(message_receipts.delivered_at, EXCLUDED.delivered_at),
              read_at = COALESCE(message_receipts.read_at, EXCLUDED.read_at)`, conversationID, userID, seq)
		}
		if err != nil {
			return 0, fmt.Errorf("upsert message receipt: %w", err)
		}
		updated += tag.RowsAffected()
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit ack transaction: %w", err)
	}
	return int32(updated), nil
}

func (r *Repository) FetchPendingOutbox(ctx context.Context, limit int, lockTTL time.Duration) ([]OutboxEvent, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("message repository is not initialized")
	}
	if limit <= 0 || limit > 256 {
		limit = 32
	}
	if lockTTL <= 0 {
		lockTTL = 30 * time.Second
	}
	rows, err := r.db.Query(ctx, `
WITH picked AS (
    SELECT event_id
    FROM outbox_events
    WHERE status = 'pending'
      AND next_attempt_at <= NOW()
      AND (locked_until IS NULL OR locked_until < NOW())
    ORDER BY created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events e
SET locked_until = NOW() + ($2::bigint * INTERVAL '1 millisecond')
FROM picked
WHERE e.event_id = picked.event_id
RETURNING e.event_id, e.aggregate_type, e.aggregate_id, e.event_type, e.routing_key, e.payload_json, e.attempts`, limit, lockTTL.Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("fetch pending outbox events: %w", err)
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		var payload []byte
		if err := rows.Scan(&event.EventID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.RoutingKey, &payload, &event.Attempts); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		event.PayloadJSON = payload
		events = append(events, event)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("scan pending outbox events: %w", err)
	}
	return events, nil
}

func (r *Repository) MarkOutboxPublished(ctx context.Context, eventID string) error {
	if r == nil || r.db == nil {
		return errors.New("message repository is not initialized")
	}
	_, err := r.db.Exec(ctx, `
UPDATE outbox_events
SET status = 'published',
    published_at = NOW(),
    locked_until = NULL,
    last_error = ''
WHERE event_id = $1`, eventID)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

func (r *Repository) MarkOutboxFailed(ctx context.Context, eventID string, publishErr error, maxAttempts int, retryDelay time.Duration) error {
	if r == nil || r.db == nil {
		return errors.New("message repository is not initialized")
	}
	if maxAttempts <= 0 {
		maxAttempts = 20
	}
	if retryDelay <= 0 {
		retryDelay = time.Second
	}
	message := "publish failed"
	if publishErr != nil {
		message = publishErr.Error()
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	_, err := r.db.Exec(ctx, `
UPDATE outbox_events
SET attempts = attempts + 1,
    status = CASE WHEN attempts + 1 >= $2 THEN 'failed' ELSE 'pending' END,
    next_attempt_at = NOW() + ($3::bigint * INTERVAL '1 millisecond'),
    locked_until = NULL,
    last_error = $4
WHERE event_id = $1`, eventID, maxAttempts, retryDelay.Milliseconds(), message)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

type pgconnTag interface {
	RowsAffected() int64
}

type rowScanner interface {
	Scan(...any) error
}

func scanMessage(scanner rowScanner) (Message, error) {
	var msg Message
	var content []byte
	if err := scanner.Scan(
		&msg.MessageID,
		&msg.ConversationID,
		&msg.Seq,
		&msg.SenderID,
		&msg.DeviceID,
		&msg.ClientMsgID,
		&msg.ClientSeq,
		&msg.ContentType,
		&content,
		&msg.CreatedAt,
	); err != nil {
		return Message{}, err
	}
	msg.ContentJSON = content
	return msg, nil
}

func validateSaveInput(input SaveMessageInput) error {
	if normalizeID(input.ConversationID) == "" ||
		normalizeID(input.SenderID) == "" ||
		normalizeID(input.DeviceID) == "" ||
		normalizeID(input.ClientMsgID) == "" {
		return fmt.Errorf("%w: conversation_id, sender_id, device_id and client_msg_id are required", ErrInvalidArgument)
	}
	if input.Seq <= 0 {
		return fmt.Errorf("%w: seq must be positive", ErrInvalidArgument)
	}
	if input.ContentType != ContentTypeText {
		return fmt.Errorf("%w: unsupported content_type", ErrInvalidArgument)
	}
	if len(input.ContentJSON) == 0 || len(input.OutboxPayload) == 0 || input.RoutingKey == "" {
		return fmt.Errorf("%w: content_json, outbox_payload and routing_key are required", ErrInvalidArgument)
	}
	return nil
}

func enrichOutboxPayload(payload []byte, eventID string, msg Message) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("decode outbox payload: %w", err)
	}
	body["event_id"] = eventID
	body["message_id"] = msg.MessageID
	body["conversation_id"] = msg.ConversationID
	body["seq"] = msg.Seq
	body["sender_id"] = msg.SenderID
	body["created_at"] = msg.CreatedAt.UTC().Format(time.RFC3339)
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode outbox payload: %w", err)
	}
	return data, nil
}

func normalizeID(value string) string {
	return strings.TrimSpace(value)
}

func normalizeSeqs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func CodeForError(err error) string {
	if errors.Is(err, ErrInvalidArgument) {
		return CodeInvalidArgument
	}
	return CodeInternal
}
