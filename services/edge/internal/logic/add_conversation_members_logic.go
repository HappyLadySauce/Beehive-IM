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

type AddConversationMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddConversationMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddConversationMembersLogic {
	return &AddConversationMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddConversationMembersLogic) AddConversationMembers(req *types.AddConversationMembersRequest, r *http.Request) (resp *types.ConversationResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.AddMembers(l.ctx, &conversationservice.AddMembersRequest{
		ConversationId: req.ConversationId,
		ActorUserId:    identity.UserID,
		Members:        conversationMemberInputs(req.Members),
	})
	if err != nil {
		l.Errorf("add conversation members rpc failed: conversation_id=%s user_id=%s error=%v", req.ConversationId, identity.UserID, err)
		return nil, err
	}

	return edgeConversationResponse(result.GetAccepted(), result.GetErrorCode(), result.GetMessage(), nil, result.GetMembers(), nil), nil
}
