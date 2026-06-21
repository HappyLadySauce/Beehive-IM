package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RemoveMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRemoveMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveMembersLogic {
	return &RemoveMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RemoveMembers marks members as removed from a conversation.
// RemoveMembers 将成员标记为已移出会话。
func (l *RemoveMembersLogic) RemoveMembers(in *pb.RemoveMembersRequest) (*pb.RemoveMembersResponse, error) {
	removed, err := l.svcCtx.Conversations.RemoveMembers(l.ctx, in.GetConversationId(), in.GetActorUserId(), in.GetUserIds())
	if err != nil {
		if isBusinessError(err) {
			return &pb.RemoveMembersResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("remove members failed: conversation_id=%s actor_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), err)
		return nil, err
	}
	return &pb.RemoveMembersResponse{
		Accepted: true,
		Message:  "members removed",
		Removed:  removed,
	}, nil
}
