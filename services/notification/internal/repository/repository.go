package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StatusPushed  = "pushed"
	StatusDropped = "dropped"
	StatusFailed  = "failed"
)

// DeliveryInput describes one online push delivery attempt.
// DeliveryInput 描述一次在线推送投递尝试。
type DeliveryInput struct {
	EventID        string
	ConversationID string
	UserID         string
	DeviceID       string
	EdgeID         string
	ConnID         string
	SessionID      string
	Status         string
	LastError      string
}

// Repository records notification delivery outcomes.
// Repository 记录通知投递结果。
type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// RecordDelivery inserts or updates one online delivery record.
// RecordDelivery 插入或更新一条在线投递记录。
func (r *Repository) RecordDelivery(ctx context.Context, input DeliveryInput) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository is not initialized")
	}
	input = normalizeDeliveryInput(input)
	if input.EventID == "" || input.UserID == "" || input.Status == "" {
		return errors.New("event_id, user_id and status are required")
	}
	if len(input.LastError) > 1000 {
		input.LastError = input.LastError[:1000]
	}
	deliveryID := uuid.NewString()
	_, err := r.db.Exec(ctx, `
INSERT INTO notification_deliveries (
    delivery_id, event_id, conversation_id, user_id, device_id, edge_id, conn_id, session_id, status, last_error
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (event_id, conn_id)
DO UPDATE SET status = EXCLUDED.status,
              last_error = EXCLUDED.last_error,
              updated_at = NOW()`,
		deliveryID,
		input.EventID,
		input.ConversationID,
		input.UserID,
		input.DeviceID,
		input.EdgeID,
		input.ConnID,
		input.SessionID,
		input.Status,
		input.LastError,
	)
	if err != nil {
		return fmt.Errorf("record notification delivery: %w", err)
	}
	return nil
}

func normalizeDeliveryInput(input DeliveryInput) DeliveryInput {
	input.EventID = strings.TrimSpace(input.EventID)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.EdgeID = strings.TrimSpace(input.EdgeID)
	input.ConnID = strings.TrimSpace(input.ConnID)
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.Status = strings.TrimSpace(input.Status)
	input.LastError = strings.TrimSpace(input.LastError)
	return input
}
