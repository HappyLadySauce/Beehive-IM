// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/HappyLadySauce/Beehive-IM/services/message/messageservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListConversationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConversationsLogic {
	return &ListConversationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListConversationsLogic) ListConversations(req *types.ListConversationsRequest, r *http.Request) (resp *types.ListConversationsResponse, err error) {
	identity, err := requestIdentity(l.svcCtx, r, false)
	if err != nil {
		return nil, err
	}
	conversations, err := l.svcCtx.Conversation.ListConversations(l.ctx, &conversationservice.ListConversationsRequest{
		UserId: identity.UserID,
		Limit:  req.Limit,
		Offset: req.Offset,
	})
	if err != nil {
		l.Errorf("list conversations rpc failed: user_id=%s error=%v", identity.UserID, err)
		return nil, err
	}
	cursors := make([]*messageservice.ConversationSummaryCursor, 0, len(conversations.GetItems()))
	for _, item := range conversations.GetItems() {
		if item == nil || item.GetConversation() == nil || item.GetMember() == nil {
			continue
		}
		member := item.GetMember()
		cursors = append(cursors, &messageservice.ConversationSummaryCursor{
			ConversationId: item.GetConversation().GetConversationId(),
			LastReadSeq:    member.GetLastReadSeq(),
			VisibleFromSeq: member.GetVisibleFromSeq(),
			VisibleToSeq:   member.GetVisibleToSeq(),
		})
	}
	summaryResp, err := l.svcCtx.Message.GetConversationSummaries(l.ctx, &messageservice.GetConversationSummariesRequest{Cursors: cursors})
	if err != nil {
		l.Errorf("conversation summaries rpc failed: user_id=%s error=%v", identity.UserID, err)
		return nil, err
	}
	if !summaryResp.GetAccepted() {
		return nil, fmt.Errorf("conversation summaries rejected: %s", summaryResp.GetMessage())
	}
	summaries := summaryMap(summaryResp.GetSummaries())
	items := make([]types.ConversationListItem, 0, len(conversations.GetItems()))
	for _, item := range conversations.GetItems() {
		if item == nil {
			continue
		}
		out := types.ConversationListItem{
			Conversation: edgeConversation(item.GetConversation()),
			Member:       edgeConversationMember(item.GetMember()),
			Settings:     edgeConversationSettings(item.GetSettings()),
			MemberCount:  item.GetMemberCount(),
		}
		if summary := summaries[out.Conversation.ConversationId]; summary != nil {
			out.LastMessage = edgeMessageItem(summary.GetLastMessage())
			out.LatestSeq = summary.GetLatestSeq()
			out.UnreadCount = summary.GetUnreadCount()
		}
		items = append(items, out)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Settings.Pinned != items[j].Settings.Pinned {
			return items[i].Settings.Pinned
		}
		left := conversationSortTime(items[i])
		right := conversationSortTime(items[j])
		if left != right {
			return left > right
		}
		return items[i].Conversation.ConversationId < items[j].Conversation.ConversationId
	})

	return &types.ListConversationsResponse{Items: items}, nil
}

func conversationSortTime(item types.ConversationListItem) string {
	if item.LastMessage.CreatedAt != "" {
		return item.LastMessage.CreatedAt
	}
	return item.Conversation.UpdatedAt
}
