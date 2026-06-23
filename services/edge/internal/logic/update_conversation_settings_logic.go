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

type UpdateConversationSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateConversationSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateConversationSettingsLogic {
	return &UpdateConversationSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateConversationSettingsLogic) UpdateConversationSettings(req *types.UpdateConversationSettingsRequest, r *http.Request) (resp *types.ConversationResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.UpdateConversationSettings(l.ctx, &conversationservice.UpdateConversationSettingsRequest{
		ConversationId: req.ConversationId,
		ActorUserId:    identity.UserID,
		TargetUserId:   identity.UserID,
		Pinned:         req.Pinned,
		MutedUntil:     req.MutedUntil,
		Remark:         req.Remark,
	})
	if err != nil {
		l.Errorf("update conversation settings rpc failed: conversation_id=%s user_id=%s error=%v", req.ConversationId, identity.UserID, err)
		return nil, err
	}

	return edgeConversationResponse(result.GetAccepted(), result.GetErrorCode(), result.GetMessage(), nil, nil, result.GetSettings()), nil
}
