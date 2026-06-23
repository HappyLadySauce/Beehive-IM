// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/HappyLadySauce/Beehive-IM/services/message/messageservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type AckMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAckMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AckMessagesLogic {
	return &AckMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AckMessagesLogic) AckMessages(req *types.AckMessagesRequest, r *http.Request) (resp *types.AckMessagesResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, true)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.Message.AckMessages(l.ctx, &messageservice.AckMessagesRequest{
		ConversationId: req.ConversationId,
		UserId:         identity.UserID,
		DeviceId:       identity.DeviceID,
		AckType:        req.AckType,
		Seqs:           req.Seqs,
	})
	if err != nil {
		l.Errorf("message ack rpc failed: conversation_id=%s user_id=%s error=%v", req.ConversationId, identity.UserID, err)
		return nil, err
	}
	return &types.AckMessagesResponse{
		Accepted:  result.GetAccepted(),
		ErrorCode: result.GetErrorCode(),
		Message:   result.GetMessage(),
		Updated:   result.GetUpdated(),
	}, nil
}
