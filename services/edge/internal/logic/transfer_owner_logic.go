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

type TransferOwnerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTransferOwnerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferOwnerLogic {
	return &TransferOwnerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TransferOwnerLogic) TransferOwner(req *types.TransferOwnerRequest, r *http.Request) (resp *types.ConversationResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.Conversation.TransferOwner(l.ctx, &conversationservice.TransferOwnerRequest{
		ConversationId: req.ConversationId,
		ActorUserId:    identity.UserID,
		TargetUserId:   req.TargetUserId,
	})
	if err != nil {
		l.Errorf("transfer owner rpc failed: conversation_id=%s user_id=%s target_user_id=%s error=%v", req.ConversationId, identity.UserID, req.TargetUserId, err)
		return nil, err
	}

	members := []*conversationservice.ConversationMember{result.GetOldOwner(), result.GetNewOwner()}
	return edgeConversationResponse(result.GetAccepted(), result.GetErrorCode(), result.GetMessage(), nil, members, nil), nil
}
