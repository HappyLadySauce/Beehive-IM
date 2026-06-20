package logic

import (
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"
)

func storeConnection(in *pb.ConnectionMeta) store.ConnectionMeta {
	if in == nil {
		return store.ConnectionMeta{}
	}
	return store.ConnectionMeta{
		SessionID:        in.GetSessionId(),
		ConnID:           in.GetConnId(),
		EdgeID:           in.GetEdgeId(),
		UserID:           in.GetUserId(),
		DeviceID:         in.GetDeviceId(),
		GatewayID:        in.GetGatewayId(),
		LastClientSeq:    in.GetLastClientSeq(),
		LastDeliveredSeq: in.GetLastDeliveredSeq(),
	}
}

func pbConnection(in store.ConnectionMeta) *pb.ConnectionMeta {
	return &pb.ConnectionMeta{
		SessionId:        in.SessionID,
		ConnId:           in.ConnID,
		EdgeId:           in.EdgeID,
		UserId:           in.UserID,
		DeviceId:         in.DeviceID,
		GatewayId:        in.GatewayID,
		LastClientSeq:    in.LastClientSeq,
		LastDeliveredSeq: in.LastDeliveredSeq,
	}
}

func ttlFromSeconds(seconds int64) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
