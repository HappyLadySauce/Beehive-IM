package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMemberRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateMemberRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMemberRoleLogic {
	return &UpdateMemberRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateMemberRole changes one member role.
// UpdateMemberRole 修改单个成员角色。
func (l *UpdateMemberRoleLogic) UpdateMemberRole(in *pb.UpdateMemberRoleRequest) (*pb.UpdateMemberRoleResponse, error) {
	member, err := l.svcCtx.Conversations.UpdateMemberRole(l.ctx, in.GetConversationId(), in.GetActorUserId(), in.GetTargetUserId(), in.GetRole())
	if err != nil {
		if isBusinessError(err) {
			return &pb.UpdateMemberRoleResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("update member role failed: conversation_id=%s actor_user_id=%s target_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), in.GetTargetUserId(), err)
		return nil, err
	}
	return &pb.UpdateMemberRoleResponse{
		Accepted: true,
		Message:  "role updated",
		Member:   memberPB(member),
	}, nil
}
