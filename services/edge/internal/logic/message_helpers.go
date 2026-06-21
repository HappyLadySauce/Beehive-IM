package logic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/HappyLadySauce/Beehive-IM/services/message/messageservice"
)

var (
	ErrMissingDebugUserID   = errors.New("missing X-Debug-User-Id header")
	ErrMissingDebugDeviceID = errors.New("missing X-Debug-Device-Id header")
)

func debugUserID(r *http.Request) (string, error) {
	if r == nil {
		return "", ErrMissingDebugUserID
	}
	userID := strings.TrimSpace(r.Header.Get("X-Debug-User-Id"))
	if userID == "" {
		return "", ErrMissingDebugUserID
	}
	return userID, nil
}

func debugDeviceID(r *http.Request) (string, error) {
	if r == nil {
		return "", ErrMissingDebugDeviceID
	}
	deviceID := strings.TrimSpace(r.Header.Get("X-Debug-Device-Id"))
	if deviceID == "" {
		return "", ErrMissingDebugDeviceID
	}
	return deviceID, nil
}

func edgeMessageItem(in *messageservice.MessageItem) types.MessageItem {
	if in == nil {
		return types.MessageItem{}
	}
	return types.MessageItem{
		MessageId:      in.GetMessageId(),
		ConversationId: in.GetConversationId(),
		Seq:            in.GetSeq(),
		SenderId:       in.GetSenderId(),
		DeviceId:       in.GetDeviceId(),
		ClientMsgId:    in.GetClientMsgId(),
		ClientSeq:      in.GetClientSeq(),
		ContentType:    in.GetContentType(),
		ContentJson:    in.GetContentJson(),
		CreatedAt:      in.GetCreatedAt(),
	}
}

func edgeMessageItems(in []*messageservice.MessageItem) []types.MessageItem {
	out := make([]types.MessageItem, 0, len(in))
	for _, item := range in {
		out = append(out, edgeMessageItem(item))
	}
	return out
}
