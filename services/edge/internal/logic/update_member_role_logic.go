// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMemberRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMemberRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMemberRoleLogic {
	return &UpdateMemberRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMemberRoleLogic) UpdateMemberRole(req *types.UpdateMemberRoleRequest, r *http.Request) (resp *types.ConversationResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.UpdateMemberRole(l.ctx, &conversationservice.UpdateMemberRoleRequest{
		ConversationId: req.ConversationId,
		ActorUserId:    identity.UserID,
		TargetUserId:   req.UserId,
		Role:           req.Role,
	})
	if err != nil {
		l.Errorf("update member role rpc failed: conversation_id=%s user_id=%s target_user_id=%s error=%v", req.ConversationId, identity.UserID, req.UserId, err)
		return nil, err
	}

	return edgeConversationResponse(result.GetAccepted(), result.GetErrorCode(), result.GetMessage(), nil, []*conversationservice.ConversationMember{result.GetMember()}, nil), nil
}
