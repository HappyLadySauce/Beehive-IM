package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateConversationSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateConversationSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateConversationSettingsLogic {
	return &UpdateConversationSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateConversationSettings updates settings owned by one user.
// UpdateConversationSettings 更新某个用户自己的会话设置。
func (l *UpdateConversationSettingsLogic) UpdateConversationSettings(in *pb.UpdateConversationSettingsRequest) (*pb.UpdateConversationSettingsResponse, error) {
	settings, err := l.svcCtx.Conversations.UpdateSettings(l.ctx, repository.UpdateSettingsInput{
		ConversationID: in.GetConversationId(),
		ActorUserID:    in.GetActorUserId(),
		TargetUserID:   in.GetTargetUserId(),
		Pinned:         in.GetPinned(),
		MutedUntil:     in.GetMutedUntil(),
		Remark:         in.GetRemark(),
	})
	if err != nil {
		if isBusinessError(err) {
			return &pb.UpdateConversationSettingsResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("update conversation settings failed: conversation_id=%s actor_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), err)
		return nil, err
	}
	return &pb.UpdateConversationSettingsResponse{
		Accepted: true,
		Message:  "settings updated",
		Settings: settingsPB(settings),
	}, nil
}
