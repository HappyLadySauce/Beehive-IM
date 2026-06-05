package message

import (
	"time"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/presence"
	"github.com/HappyLadySauce/Beehive-IM/pkg/mq"
	"gorm.io/gorm"
)

// MessageService orchestrates message persistence and MQ fan-out.
// MessageService 负责消息落库与 MQ 扇出。
type MessageService struct {
	DB                  *gorm.DB
	MQ                  *mq.Client
	Presence            *presence.Service
	PublishTimeout      time.Duration
	PublishBatchSize    int
	DeliveryMaxAttempts int
	PublishScanInterval time.Duration
}

// NewMessageService creates a new MessageService.
// NewMessageService 创建一个新的 MessageService。
func NewMessageService(db *gorm.DB, mqClient *mq.Client, presenceService *presence.Service, publishTimeout time.Duration, publishBatchSize, deliveryMaxAttempts int) *MessageService {
	if publishBatchSize <= 0 {
		publishBatchSize = 100
	}
	if deliveryMaxAttempts <= 0 {
		deliveryMaxAttempts = 5
	}
	return &MessageService{
		DB:                  db,
		MQ:                  mqClient,
		Presence:            presenceService,
		PublishTimeout:      publishTimeout,
		PublishBatchSize:    publishBatchSize,
		DeliveryMaxAttempts: deliveryMaxAttempts,
		PublishScanInterval: time.Second,
	}
}
