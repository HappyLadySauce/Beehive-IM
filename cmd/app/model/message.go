package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	ConversationTypeDirect = "direct"
	ConversationTypeGroup  = "group"

	MessageStatusCreated = "created"

	DeliveryStatusPending    = "pending"
	DeliveryStatusPublished  = "published"
	DeliveryStatusDispatched = "dispatched"
	DeliveryStatusDelivered  = "delivered"
	DeliveryStatusFailed     = "failed"
)

// Conversation stores chat conversation metadata.
// Conversation 保存聊天会话元数据。
type Conversation struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	Type      string         `gorm:"type:varchar(20);not null;default:'direct'" json:"type"`
	Title     *string        `gorm:"type:varchar(255)" json:"title,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Conversation) TableName() string {
	return "conversations"
}

// ConversationMember stores one user's membership in a conversation.
// ConversationMember 保存用户在会话中的成员关系。
type ConversationMember struct {
	ID             uint64         `gorm:"primaryKey" json:"id"`
	ConversationID uint64         `gorm:"not null;uniqueIndex:idx_conversation_member_unique" json:"conversation_id"`
	UserID         uint64         `gorm:"not null;uniqueIndex:idx_conversation_member_unique" json:"user_id"`
	Role           string         `gorm:"type:varchar(20);not null;default:'member'" json:"role"`
	JoinedAt       time.Time      `json:"joined_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ConversationMember) TableName() string {
	return "conversation_members"
}

// Message stores immutable text message content and idempotency metadata.
// Message 保存不可变文本消息内容及幂等元数据。
type Message struct {
	ID              uint64         `gorm:"primaryKey" json:"id"`
	MessageID       string         `gorm:"type:varchar(64);not null;uniqueIndex" json:"message_id"`
	ConversationID  uint64         `gorm:"not null;uniqueIndex:idx_message_conversation_sequence" json:"conversation_id"`
	SenderUserID    uint64         `gorm:"not null" json:"sender_user_id"`
	ClientMessageID string         `gorm:"type:varchar(128);not null;uniqueIndex:idx_message_sender_client" json:"client_message_id"`
	Content         string         `gorm:"type:text;not null" json:"content"`
	Status          string         `gorm:"type:varchar(20);not null;default:'created'" json:"status"`
	Sequence        uint64         `gorm:"not null;uniqueIndex:idx_message_conversation_sequence" json:"sequence"`
	SentAt          time.Time      `json:"sent_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Message) TableName() string {
	return "messages"
}

// MessageDelivery tracks delivery state for one recipient.
// MessageDelivery 记录单个接收人的消息投递状态。
type MessageDelivery struct {
	ID              uint64         `gorm:"primaryKey" json:"id"`
	MessageID       string         `gorm:"type:varchar(64);not null;uniqueIndex:idx_delivery_message_recipient" json:"message_id"`
	ConversationID  uint64         `gorm:"not null" json:"conversation_id"`
	RecipientUserID uint64         `gorm:"not null;uniqueIndex:idx_delivery_message_recipient" json:"recipient_user_id"`
	Status          string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	AttemptCount    int            `gorm:"not null;default:0" json:"attempt_count"`
	PublishedAt     *time.Time     `json:"published_at,omitempty"`
	DispatchedAt    *time.Time     `json:"dispatched_at,omitempty"`
	DeliveredAt     *time.Time     `json:"delivered_at,omitempty"`
	LastError       *string        `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MessageDelivery) TableName() string {
	return "message_deliveries"
}
