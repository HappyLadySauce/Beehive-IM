package message

import (
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/pkg/mq"
)

// MessageService is the service for managing messages.
// MessageService 是管理消息的服务。
type MessageService struct {
	MQ *mq.Client
}

// NewMessageService creates a new MessageService.
// NewMessageService 创建一个新的 MessageService。
func NewMessageService(svcCtx *svc.ServiceContext) *MessageService {
	return &MessageService{
		MQ: svcCtx.MQ,
	}
}
