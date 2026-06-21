package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListConversationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConversationsLogic {
	return &ListConversationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListConversations returns conversations joined by one user.
// ListConversations 返回某个用户加入的会话列表。
func (l *ListConversationsLogic) ListConversations(in *pb.ListConversationsRequest) (*pb.ListConversationsResponse, error) {
	conversations, err := l.svcCtx.Conversations.List(l.ctx, in.GetUserId(), in.GetLimit(), in.GetOffset())
	if err != nil {
		l.Errorf("list conversations failed: user_id=%s error=%v", in.GetUserId(), err)
		return nil, err
	}
	return &pb.ListConversationsResponse{Conversations: conversationsPB(conversations)}, nil
}
