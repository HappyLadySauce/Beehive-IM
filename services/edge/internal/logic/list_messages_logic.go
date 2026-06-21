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

type ListMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMessagesLogic {
	return &ListMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListMessagesLogic) ListMessages(req *types.ListMessagesRequest, r *http.Request) (resp *types.ListMessagesResponse, err error) {
	userID, err := debugUserID(r)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.Message.ListMessages(l.ctx, &messageservice.ListMessagesRequest{
		ConversationId: req.ConversationId,
		UserId:         userID,
		AfterSeq:       req.AfterSeq,
		BeforeSeq:      req.BeforeSeq,
		Direction:      req.Direction,
		Limit:          req.Limit,
	})
	if err != nil {
		l.Errorf("message list rpc failed: conversation_id=%s user_id=%s error=%v", req.ConversationId, userID, err)
		return nil, err
	}
	return &types.ListMessagesResponse{
		Accepted:  result.GetAccepted(),
		ErrorCode: result.GetErrorCode(),
		Message:   result.GetMessage(),
		Messages:  edgeMessageItems(result.GetMessages()),
		LatestSeq: result.GetLatestSeq(),
	}, nil
}
