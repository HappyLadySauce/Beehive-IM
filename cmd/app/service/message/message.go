package message

import (
	"github.com/HappyLadySauce/Beehive-IM/pkg/mq"
	"gorm.io/gorm"
)

// MessageService orchestrates message persistence and MQ fan-out.
// MessageService 负责消息落库与 MQ 扇出。
type MessageService struct {
	DB *gorm.DB
	MQ *mq.Client
}

// NewMessageService creates a new MessageService.
// NewMessageService 创建一个新的 MessageService。
func NewMessageService(db *gorm.DB, mqClient *mq.Client) *MessageService {
	return &MessageService{
		DB: db,
		MQ: mqClient,
	}
}
