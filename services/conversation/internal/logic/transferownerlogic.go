package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TransferOwnerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransferOwnerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferOwnerLogic {
	return &TransferOwnerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// TransferOwner transfers group ownership to another active member.
func (l *TransferOwnerLogic) TransferOwner(in *pb.TransferOwnerRequest) (*pb.TransferOwnerResponse, error) {
	oldOwner, newOwner, err := l.svcCtx.Conversations.TransferOwner(l.ctx, in.GetConversationId(), in.GetActorUserId(), in.GetTargetUserId())
	if err != nil {
		if isBusinessError(err) {
			return &pb.TransferOwnerResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("transfer owner failed: conversation_id=%s actor_user_id=%s target_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), in.GetTargetUserId(), err)
		return nil, err
	}

	return &pb.TransferOwnerResponse{
		Accepted: true,
		Message:  "owner transferred",
		OldOwner: memberPB(oldOwner),
		NewOwner: memberPB(newOwner),
	}, nil
}
