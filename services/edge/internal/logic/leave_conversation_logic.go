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

type LeaveConversationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLeaveConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LeaveConversationLogic {
	return &LeaveConversationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LeaveConversationLogic) LeaveConversation(req *types.ConversationActionRequest, r *http.Request) (resp *types.EmptyResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.LeaveConversation(l.ctx, &conversationservice.LeaveConversationRequest{
		ConversationId: req.ConversationId,
		ActorUserId:    identity.UserID,
	})
	if err != nil {
		l.Errorf("leave conversation rpc failed: conversation_id=%s user_id=%s error=%v", req.ConversationId, identity.UserID, err)
		return nil, err
	}

	return emptyFromAccepted(result.GetAccepted(), result.GetErrorCode(), result.GetMessage()), nil
}
