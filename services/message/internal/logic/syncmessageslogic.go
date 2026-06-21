package logic

import (
	"context"
	"strings"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/message/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

const maxSyncConversations = 50

type SyncMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSyncMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncMessagesLogic {
	return &SyncMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SyncMessages returns missing messages for multiple conversations.
// SyncMessages 返回多个会话的缺口消息。
func (l *SyncMessagesLogic) SyncMessages(in *pb.SyncMessagesRequest) (*pb.SyncMessagesResponse, error) {
	userID := strings.TrimSpace(in.GetUserId())
	if userID == "" {
		return syncRejected(repository.CodeInvalidArgument, "user_id is required"), nil
	}
	cursors := normalizeCursors(in.GetCursors())
	if len(cursors) > maxSyncConversations {
		return syncRejected(repository.CodeInvalidArgument, "too many conversation cursors"), nil
	}

	results := make([]*pb.ConversationSyncResult, 0, len(cursors))
	for _, cursor := range cursors {
		permission, err := l.svcCtx.Conversation.CheckReadPermission(l.ctx, &conversationservice.CheckReadPermissionRequest{
			ConversationId: cursor.ConversationID,
			UserId:         userID,
		})
		if err != nil {
			l.Errorf("check sync messages permission rpc failed: conversation_id=%s user_id=%s error=%v", cursor.ConversationID, userID, err)
			return nil, err
		}
		if !permission.GetAllowed() {
			return syncRejected(permission.GetErrorCode(), permission.GetMessage()), nil
		}

		messages, latestSeq, err := l.svcCtx.Messages.ListMessages(
			l.ctx,
			cursor.ConversationID,
			cursor.LastSeq,
			0,
			repository.DirectionForward,
			in.GetLimitPerConversation(),
		)
		if err != nil {
			if isBusinessError(err) {
				return syncRejected(repository.CodeForError(err), err.Error()), nil
			}
			l.Errorf("sync messages failed: conversation_id=%s user_id=%s error=%v", cursor.ConversationID, userID, err)
			return nil, err
		}
		results = append(results, &pb.ConversationSyncResult{
			ConversationId: cursor.ConversationID,
			Messages:       messageItemsPB(messages),
			LatestSeq:      latestSeq,
		})
	}
	return &pb.SyncMessagesResponse{
		Accepted:      true,
		Message:       "synced",
		Conversations: results,
	}, nil
}

type syncCursor struct {
	ConversationID string
	LastSeq        int64
}

func normalizeCursors(inputs []*pb.ConversationCursor) []syncCursor {
	seen := make(map[string]struct{}, len(inputs))
	out := make([]syncCursor, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			continue
		}
		conversationID := strings.TrimSpace(input.GetConversationId())
		if conversationID == "" {
			continue
		}
		if _, ok := seen[conversationID]; ok {
			continue
		}
		seen[conversationID] = struct{}{}
		lastSeq := input.GetLastSeq()
		if lastSeq < 0 {
			lastSeq = 0
		}
		out = append(out, syncCursor{
			ConversationID: conversationID,
			LastSeq:        lastSeq,
		})
	}
	return out
}
