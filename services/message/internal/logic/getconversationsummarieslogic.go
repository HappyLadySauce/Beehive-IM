package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/message/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationSummariesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationSummariesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationSummariesLogic {
	return &GetConversationSummariesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetConversationSummaries returns last message and unread count for conversations.
func (l *GetConversationSummariesLogic) GetConversationSummaries(in *pb.GetConversationSummariesRequest) (*pb.GetConversationSummariesResponse, error) {
	cursors := make([]repository.SummaryCursor, 0, len(in.GetCursors()))
	for _, cursor := range in.GetCursors() {
		if cursor == nil {
			continue
		}
		cursors = append(cursors, repository.SummaryCursor{
			ConversationID: cursor.GetConversationId(),
			LastReadSeq:    cursor.GetLastReadSeq(),
			VisibleFromSeq: cursor.GetVisibleFromSeq(),
			VisibleToSeq:   cursor.GetVisibleToSeq(),
		})
	}
	summaries, err := l.svcCtx.Messages.GetConversationSummaries(l.ctx, cursors)
	if err != nil {
		if isBusinessError(err) {
			return summariesRejected(repository.CodeForError(err), err.Error()), nil
		}
		l.Errorf("get conversation summaries failed: count=%d error=%v", len(cursors), err)
		return nil, err
	}

	return &pb.GetConversationSummariesResponse{
		Accepted:  true,
		Message:   "summarized",
		Summaries: summariesPB(summaries),
	}, nil
}
