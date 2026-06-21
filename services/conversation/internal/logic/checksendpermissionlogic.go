package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckSendPermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckSendPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckSendPermissionLogic {
	return &CheckSendPermissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CheckSendPermission checks whether a user can send to a conversation.
// CheckSendPermission 校验用户是否可向会话发送消息。
func (l *CheckSendPermissionLogic) CheckSendPermission(in *pb.CheckSendPermissionRequest) (*pb.CheckSendPermissionResponse, error) {
	if err := l.svcCtx.Conversations.CheckSendPermission(l.ctx, in.GetConversationId(), in.GetUserId()); err != nil {
		if isBusinessError(err) {
			return &pb.CheckSendPermissionResponse{
				Allowed:   false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("check send permission failed: conversation_id=%s user_id=%s error=%v", in.GetConversationId(), in.GetUserId(), err)
		return nil, err
	}
	return &pb.CheckSendPermissionResponse{
		Allowed: true,
		Message: "allowed",
	}, nil
}
