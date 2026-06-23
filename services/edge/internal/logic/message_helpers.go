package logic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/HappyLadySauce/Beehive-IM/services/message/messageservice"
)

var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrMissingDeviceID = errors.New("missing device id")
)

type RequestIdentity struct {
	UserID   string
	DeviceID string
}

func requestIdentity(svcCtx *svc.ServiceContext, r *http.Request, requireDevice bool) (RequestIdentity, error) {
	userID, err := authenticatedUserID(svcCtx, r)
	if err != nil {
		return RequestIdentity{}, err
	}
	deviceID := requestDeviceID(r)
	if requireDevice && deviceID == "" {
		return RequestIdentity{}, ErrMissingDeviceID
	}
	return RequestIdentity{UserID: userID, DeviceID: deviceID}, nil
}

func authenticatedUserID(svcCtx *svc.ServiceContext, r *http.Request) (string, error) {
	if r == nil {
		return "", ErrUnauthorized
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return "", ErrUnauthorized
		}
		claims, err := svcCtx.JWT.Verify(parts[1])
		if err != nil {
			return "", ErrUnauthorized
		}
		return strings.TrimSpace(claims.Subject), nil
	}
	if devAuthAllowed(svcCtx) {
		userID := strings.TrimSpace(r.Header.Get("X-Debug-User-Id"))
		if userID != "" {
			return userID, nil
		}
	}
	return "", ErrUnauthorized
}

func requestDeviceID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if deviceID := strings.TrimSpace(r.Header.Get("X-Device-Id")); deviceID != "" {
		return deviceID
	}
	return strings.TrimSpace(r.Header.Get("X-Debug-Device-Id"))
}

func devAuthAllowed(svcCtx *svc.ServiceContext) bool {
	if svcCtx == nil || !svcCtx.Config.DevAuth.Enabled {
		return false
	}
	env := strings.ToLower(strings.TrimSpace(svcCtx.Config.Env))
	return env == "dev" || env == "test"
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
