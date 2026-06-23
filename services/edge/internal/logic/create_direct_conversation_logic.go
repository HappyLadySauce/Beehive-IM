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

type CreateDirectConversationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateDirectConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateDirectConversationLogic {
	return &CreateDirectConversationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateDirectConversationLogic) CreateDirectConversation(req *types.CreateDirectConversationRequest, r *http.Request) (resp *types.ConversationResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.CreateConversation(l.ctx, &conversationservice.CreateConversationRequest{
		CreatorUserId: identity.UserID,
		Type:          "direct",
		Title:         req.Title,
		Members: []*conversationservice.MemberInput{
			{UserId: req.PeerUserId, Role: "member"},
		},
	})
	if err != nil {
		l.Errorf("create direct conversation rpc failed: user_id=%s peer_user_id=%s error=%v", identity.UserID, req.PeerUserId, err)
		return nil, err
	}

	return edgeConversationResponse(result.GetAccepted(), result.GetErrorCode(), result.GetMessage(), result.GetConversation(), result.GetMembers(), nil), nil
}
