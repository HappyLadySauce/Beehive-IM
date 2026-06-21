package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationLogic {
	return &GetConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetConversation returns one conversation with members and user settings.
// GetConversation 返回单个会话、成员和用户设置。
func (l *GetConversationLogic) GetConversation(in *pb.GetConversationRequest) (*pb.GetConversationResponse, error) {
	conversation, members, settings, err := l.svcCtx.Conversations.Get(l.ctx, in.GetConversationId(), in.GetRequesterUserId())
	if err != nil {
		if isBusinessError(err) {
			return &pb.GetConversationResponse{
				Found:     false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("get conversation failed: conversation_id=%s requester_user_id=%s error=%v", in.GetConversationId(), in.GetRequesterUserId(), err)
		return nil, err
	}
	return &pb.GetConversationResponse{
		Found:        true,
		Message:      "ok",
		Conversation: conversationPB(conversation),
		Members:      membersPB(members),
		Settings:     settingsPB(settings),
	}, nil
}
