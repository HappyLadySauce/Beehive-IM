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

type CreateGroupConversationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateGroupConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGroupConversationLogic {
	return &CreateGroupConversationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateGroupConversationLogic) CreateGroupConversation(req *types.CreateGroupConversationRequest, r *http.Request) (resp *types.ConversationResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.CreateConversation(l.ctx, &conversationservice.CreateConversationRequest{
		CreatorUserId: identity.UserID,
		Type:          "group",
		Title:         req.Title,
		Members:       conversationMemberInputs(req.Members),
	})
	if err != nil {
		l.Errorf("create group conversation rpc failed: user_id=%s error=%v", identity.UserID, err)
		return nil, err
	}

	return edgeConversationResponse(result.GetAccepted(), result.GetErrorCode(), result.GetMessage(), result.GetConversation(), result.GetMembers(), nil), nil
}
