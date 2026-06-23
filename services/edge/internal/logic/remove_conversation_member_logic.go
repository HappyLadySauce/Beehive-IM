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

type RemoveConversationMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRemoveConversationMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveConversationMemberLogic {
	return &RemoveConversationMemberLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RemoveConversationMemberLogic) RemoveConversationMember(req *types.RemoveConversationMemberRequest, r *http.Request) (resp *types.EmptyResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.RemoveMembers(l.ctx, &conversationservice.RemoveMembersRequest{
		ConversationId: req.ConversationId,
		ActorUserId:    identity.UserID,
		UserIds:        []string{req.UserId},
	})
	if err != nil {
		l.Errorf("remove conversation member rpc failed: conversation_id=%s user_id=%s target_user_id=%s error=%v", req.ConversationId, identity.UserID, req.UserId, err)
		return nil, err
	}

	return emptyFromAccepted(result.GetAccepted(), result.GetErrorCode(), result.GetMessage()), nil
}
