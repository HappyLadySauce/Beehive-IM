package message

import (
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"

)

type MessageService struct {

}

func NewMessageService(s *svc.ServiceContext) *MessageService {
	return &MessageService{
	}
}
