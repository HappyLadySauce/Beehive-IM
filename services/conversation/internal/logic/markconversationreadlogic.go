package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkConversationReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkConversationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkConversationReadLogic {
	return &MarkConversationReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MarkConversationRead advances one member read or delivered cursor.
func (l *MarkConversationReadLogic) MarkConversationRead(in *pb.MarkConversationReadRequest) (*pb.MarkConversationReadResponse, error) {
	member, err := l.svcCtx.Conversations.MarkRead(l.ctx, in.GetConversationId(), in.GetUserId(), in.GetCursorType(), in.GetSeq())
	if err != nil {
		if isBusinessError(err) {
			return &pb.MarkConversationReadResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("mark conversation read failed: conversation_id=%s user_id=%s cursor_type=%s seq=%d error=%v", in.GetConversationId(), in.GetUserId(), in.GetCursorType(), in.GetSeq(), err)
		return nil, err
	}

	return &pb.MarkConversationReadResponse{
		Accepted:         true,
		Message:          "marked",
		LastReadSeq:      member.LastReadSeq,
		LastDeliveredSeq: member.LastDeliveredSeq,
	}, nil
}
