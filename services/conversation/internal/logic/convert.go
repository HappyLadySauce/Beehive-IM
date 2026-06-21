package logic

import (
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"
)

const timeFormat = time.RFC3339

func conversationPB(c repository.Conversation) *pb.Conversation {
	return &pb.Conversation{
		ConversationId: c.ConversationID,
		Type:           c.Type,
		Status:         c.Status,
		Title:          c.Title,
		OwnerUserId:    c.OwnerUserID,
		CurrentSeq:     c.CurrentSeq,
		CreatedAt:      formatTime(c.CreatedAt),
		UpdatedAt:      formatTime(c.UpdatedAt),
	}
}

func memberPB(m repository.Member) *pb.ConversationMember {
	return &pb.ConversationMember{
		ConversationId: m.ConversationID,
		UserId:         m.UserID,
		Role:           m.Role,
		Status:         m.Status,
		MutedUntil:     formatOptionalTime(m.MutedUntil),
		JoinedAt:       formatTime(m.JoinedAt),
		UpdatedAt:      formatTime(m.UpdatedAt),
	}
}

func membersPB(members []repository.Member) []*pb.ConversationMember {
	out := make([]*pb.ConversationMember, 0, len(members))
	for _, member := range members {
		out = append(out, memberPB(member))
	}
	return out
}

func settingsPB(s repository.Settings) *pb.ConversationSettings {
	return &pb.ConversationSettings{
		ConversationId: s.ConversationID,
		UserId:         s.UserID,
		Pinned:         s.Pinned,
		MutedUntil:     formatOptionalTime(s.MutedUntil),
		Remark:         s.Remark,
		UpdatedAt:      formatTime(s.UpdatedAt),
	}
}

func memberInputsPB(inputs []*pb.MemberInput) []repository.MemberInput {
	out := make([]repository.MemberInput, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			continue
		}
		out = append(out, repository.MemberInput{
			UserID: input.GetUserId(),
			Role:   input.GetRole(),
		})
	}
	return out
}

func conversationsPB(conversations []repository.Conversation) []*pb.Conversation {
	out := make([]*pb.Conversation, 0, len(conversations))
	for _, conversation := range conversations {
		out = append(out, conversationPB(conversation))
	}
	return out
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeFormat)
}

func formatOptionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeFormat)
}

func isBusinessError(err error) bool {
	code := repository.CodeForError(err)
	return code != repository.CodeInternal
}
