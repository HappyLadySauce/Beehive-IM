package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddMembersLogic {
	return &AddMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AddMembers adds or reactivates members in a conversation.
// AddMembers 在会话中新增或重新激活成员。
func (l *AddMembersLogic) AddMembers(in *pb.AddMembersRequest) (*pb.AddMembersResponse, error) {
	if err := validateUsersExist(l.ctx, l.svcCtx.User, userIDsFromMembers(in.GetMembers())); err != nil {
		if isBusinessError(err) {
			return &pb.AddMembersResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("validate add members users failed: conversation_id=%s actor_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), err)
		return nil, err
	}
	members, err := l.svcCtx.Conversations.AddMembers(l.ctx, in.GetConversationId(), in.GetActorUserId(), memberInputsPB(in.GetMembers()))
	if err != nil {
		if isBusinessError(err) {
			return &pb.AddMembersResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("add members failed: conversation_id=%s actor_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), err)
		return nil, err
	}
	return &pb.AddMembersResponse{
		Accepted: true,
		Message:  "members added",
		Members:  membersPB(members),
	}, nil
}
