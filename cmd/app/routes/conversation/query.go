package conversation

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	msgsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

// ListConversations returns conversations visible to the authenticated user.
// ListConversations 返回认证用户可见的会话列表。
func (c *Controller) ListConversations() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		items, err := c.svc.MessageStore.ListConversationsForUser(ctx.Request.Context(), ctx.GetString("userID"))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list conversations"})
			return
		}
		resp := v1.ListConversationsResponse{Items: make([]v1.ConversationResponse, 0, len(items))}
		for _, item := range items {
			resp.Items = append(resp.Items, v1.ConversationResponse{
				ID:        item.ID,
				Type:      item.Type,
				Title:     item.Title,
				CreatedAt: item.CreatedAt,
				UpdatedAt: item.UpdatedAt,
			})
		}
		ctx.JSON(http.StatusOK, resp)
	}
}

// ListMessages returns paginated message history for one conversation.
// ListMessages 返回单个会话的分页历史消息。
func (c *Controller) ListMessages() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		before, err := parseOptionalUint(ctx.Query("before"))
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid before"})
			return
		}
		limit, err := parseOptionalInt(ctx.Query("limit"))
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}

		items, err := c.svc.MessageStore.ListMessages(ctx.Request.Context(), ctx.GetString("userID"), ctx.Param("id"), before, limit)
		if err != nil {
			if errors.Is(err, msgsvc.ErrConversationForbidden) {
				ctx.JSON(http.StatusForbidden, gin.H{"error": "conversation access denied"})
				return
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list messages"})
			return
		}
		resp := v1.ListMessagesResponse{Items: make([]v1.MessageResponseItem, 0, len(items))}
		for _, item := range items {
			resp.Items = append(resp.Items, v1.MessageResponseItem{
				MessageID:       item.MessageID,
				ClientMessageID: item.ClientMessageID,
				ConversationID:  item.ConversationID,
				FromUserID:      item.FromUserID,
				Content:         item.Content,
				Sequence:        item.Sequence,
				SentAt:          item.SentAt,
			})
		}
		ctx.JSON(http.StatusOK, resp)
	}
}

func parseOptionalUint(raw string) (uint64, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func parseOptionalInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}
