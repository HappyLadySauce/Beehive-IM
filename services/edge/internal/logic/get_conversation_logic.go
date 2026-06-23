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

type GetConversationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationLogic {
	return &GetConversationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConversationLogic) GetConversation(req *types.GetConversationRequest, r *http.Request) (resp *types.ConversationResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.GetConversation(l.ctx, &conversationservice.GetConversationRequest{
		ConversationId:  req.ConversationId,
		RequesterUserId: identity.UserID,
	})
	if err != nil {
		l.Errorf("get conversation rpc failed: conversation_id=%s user_id=%s error=%v", req.ConversationId, identity.UserID, err)
		return nil, err
	}

	return edgeConversationResponse(result.GetFound(), result.GetErrorCode(), result.GetMessage(), result.GetConversation(), result.GetMembers(), result.GetSettings()), nil
}
