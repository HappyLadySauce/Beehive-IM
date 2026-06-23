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

type SyncMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSyncMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncMessagesLogic {
	return &SyncMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SyncMessagesLogic) SyncMessages(req *types.SyncMessagesRequest, r *http.Request) (resp *types.SyncMessagesResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}

	cursors := make([]*messageservice.ConversationCursor, 0, len(req.Cursors))
	for _, cursor := range req.Cursors {
		cursors = append(cursors, &messageservice.ConversationCursor{
			ConversationId: cursor.ConversationId,
			LastSeq:        cursor.LastSeq,
		})
	}
	result, err := l.svcCtx.Message.SyncMessages(l.ctx, &messageservice.SyncMessagesRequest{
		UserId:               identity.UserID,
		Cursors:              cursors,
		LimitPerConversation: req.LimitPerConversation,
	})
	if err != nil {
		l.Errorf("message sync rpc failed: user_id=%s error=%v", identity.UserID, err)
		return nil, err
	}
	conversations := make([]types.ConversationSyncResult, 0, len(result.GetConversations()))
	for _, item := range result.GetConversations() {
		if item == nil {
			continue
		}
		conversations = append(conversations, types.ConversationSyncResult{
			ConversationId: item.GetConversationId(),
			Messages:       edgeMessageItems(item.GetMessages()),
			LatestSeq:      item.GetLatestSeq(),
		})
	}
	return &types.SyncMessagesResponse{
		Accepted:      result.GetAccepted(),
		ErrorCode:     result.GetErrorCode(),
		Message:       result.GetMessage(),
		Conversations: conversations,
	}, nil
}
