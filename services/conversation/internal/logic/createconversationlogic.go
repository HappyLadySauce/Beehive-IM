package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateConversationLogic {
	return &CreateConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateConversation creates a conversation and initial members.
// CreateConversation 创建会话和初始成员。
func (l *CreateConversationLogic) CreateConversation(in *pb.CreateConversationRequest) (*pb.CreateConversationResponse, error) {
	if err := validateUsersExist(l.ctx, l.svcCtx.User, userIDsFromCreate(in.GetCreatorUserId(), in.GetMembers())); err != nil {
		if isBusinessError(err) {
			return &pb.CreateConversationResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("validate create conversation users failed: creator_user_id=%s error=%v", in.GetCreatorUserId(), err)
		return nil, err
	}
	conversation, members, err := l.svcCtx.Conversations.Create(l.ctx, repository.CreateInput{
		CreatorUserID: in.GetCreatorUserId(),
		Type:          in.GetType(),
		Title:         in.GetTitle(),
		Members:       memberInputsPB(in.GetMembers()),
	})
	if err != nil {
		if isBusinessError(err) {
			return &pb.CreateConversationResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("create conversation failed: creator_user_id=%s error=%v", in.GetCreatorUserId(), err)
		return nil, err
	}
	return &pb.CreateConversationResponse{
		Accepted:     true,
		Message:      "created",
		Conversation: conversationPB(conversation),
		Members:      membersPB(members),
	}, nil
}
