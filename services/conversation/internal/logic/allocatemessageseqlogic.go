package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AllocateMessageSeqLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAllocateMessageSeqLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AllocateMessageSeqLogic {
	return &AllocateMessageSeqLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AllocateMessageSeq atomically allocates the next conversation message sequence.
// AllocateMessageSeq 原子分配下一个会话消息序列号。
func (l *AllocateMessageSeqLogic) AllocateMessageSeq(in *pb.AllocateMessageSeqRequest) (*pb.AllocateMessageSeqResponse, error) {
	seq, err := l.svcCtx.Conversations.AllocateSeq(l.ctx, in.GetConversationId(), in.GetUserId())
	if err != nil {
		if isBusinessError(err) {
			return &pb.AllocateMessageSeqResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("allocate message sequence failed: conversation_id=%s user_id=%s error=%v", in.GetConversationId(), in.GetUserId(), err)
		return nil, err
	}
	return &pb.AllocateMessageSeqResponse{
		Accepted: true,
		Message:  "allocated",
		Seq:      seq,
	}, nil
}
