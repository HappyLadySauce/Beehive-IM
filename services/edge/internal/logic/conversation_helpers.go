package logic

import (
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/HappyLadySauce/Beehive-IM/services/message/messageservice"
)

func edgeConversation(in *conversationservice.Conversation) types.Conversation {
	if in == nil {
		return types.Conversation{}
	}
	return types.Conversation{
		ConversationId: in.GetConversationId(),
		Type:           in.GetType(),
		Status:         in.GetStatus(),
		Title:          in.GetTitle(),
		OwnerUserId:    in.GetOwnerUserId(),
		CurrentSeq:     in.GetCurrentSeq(),
		CreatedAt:      in.GetCreatedAt(),
		UpdatedAt:      in.GetUpdatedAt(),
	}
}

func edgeConversationMember(in *conversationservice.ConversationMember) types.ConversationMember {
	if in == nil {
		return types.ConversationMember{}
	}
	return types.ConversationMember{
		ConversationId:   in.GetConversationId(),
		UserId:           in.GetUserId(),
		Role:             in.GetRole(),
		Status:           in.GetStatus(),
		MutedUntil:       in.GetMutedUntil(),
		JoinedAt:         in.GetJoinedAt(),
		UpdatedAt:        in.GetUpdatedAt(),
		VisibleFromSeq:   in.GetVisibleFromSeq(),
		VisibleToSeq:     in.GetVisibleToSeq(),
		LastReadSeq:      in.GetLastReadSeq(),
		LastDeliveredSeq: in.GetLastDeliveredSeq(),
		LastReadAt:       in.GetLastReadAt(),
	}
}

func edgeConversationMembers(in []*conversationservice.ConversationMember) []types.ConversationMember {
	out := make([]types.ConversationMember, 0, len(in))
	for _, member := range in {
		if member == nil {
			continue
		}
		out = append(out, edgeConversationMember(member))
	}
	return out
}

func edgeConversationSettings(in *conversationservice.ConversationSettings) types.ConversationSettings {
	if in == nil {
		return types.ConversationSettings{}
	}
	return types.ConversationSettings{
		ConversationId: in.GetConversationId(),
		UserId:         in.GetUserId(),
		Pinned:         in.GetPinned(),
		MutedUntil:     in.GetMutedUntil(),
		Remark:         in.GetRemark(),
		UpdatedAt:      in.GetUpdatedAt(),
	}
}

func conversationMemberInputs(in []types.MemberInput) []*conversationservice.MemberInput {
	out := make([]*conversationservice.MemberInput, 0, len(in))
	for _, member := range in {
		out = append(out, &conversationservice.MemberInput{
			UserId: member.UserId,
			Role:   member.Role,
		})
	}
	return out
}

func edgeConversationResponse(accepted bool, code, message string, conversation *conversationservice.Conversation, members []*conversationservice.ConversationMember, settings *conversationservice.ConversationSettings) *types.ConversationResponse {
	return &types.ConversationResponse{
		Accepted:     accepted,
		ErrorCode:    code,
		Message:      message,
		Conversation: edgeConversation(conversation),
		Members:      edgeConversationMembers(members),
		Settings:     edgeConversationSettings(settings),
	}
}

func emptyFromAccepted(accepted bool, code, message string) *types.EmptyResponse {
	return &types.EmptyResponse{
		Success:   accepted,
		ErrorCode: code,
		Message:   message,
	}
}

func summaryMap(in []*messageservice.ConversationSummary) map[string]*messageservice.ConversationSummary {
	out := make(map[string]*messageservice.ConversationSummary, len(in))
	for _, item := range in {
		if item != nil {
			out[item.GetConversationId()] = item
		}
	}
	return out
}
