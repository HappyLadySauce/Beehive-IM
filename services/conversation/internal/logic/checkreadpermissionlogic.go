package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckReadPermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckReadPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckReadPermissionLogic {
	return &CheckReadPermissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CheckReadPermission checks whether a user can read a conversation.
// CheckReadPermission 校验用户是否可读取会话消息。
func (l *CheckReadPermissionLogic) CheckReadPermission(in *pb.CheckReadPermissionRequest) (*pb.CheckReadPermissionResponse, error) {
	if err := l.svcCtx.Conversations.CheckReadPermission(l.ctx, in.GetConversationId(), in.GetUserId()); err != nil {
		if isBusinessError(err) {
			return &pb.CheckReadPermissionResponse{
				Allowed:   false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("check read permission failed: conversation_id=%s user_id=%s error=%v", in.GetConversationId(), in.GetUserId(), err)
		return nil, err
	}
	return &pb.CheckReadPermissionResponse{
		Allowed: true,
		Message: "allowed",
	}, nil
}
