package logic

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type StreamLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStreamLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamLogic {
	return &StreamLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Stream proxies realtime frames between Edge and Gateway.
// Stream 在 Edge 和 Gateway 之间代理实时帧。
func (l *StreamLogic) Stream(stream pb.GatewayService_StreamServer) error {
	for {
		frame, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			l.Errorf("gateway stream receive failed: %v", err)
			return err
		}

		resp := l.handleFrame(frame)
		if err := stream.Send(resp); err != nil {
			l.Errorf("gateway stream send failed: session_id=%s conn_id=%s error=%v", frame.GetSessionId(), frame.GetConnId(), err)
			return err
		}
	}
}

func (l *StreamLogic) handleFrame(frame *pb.GatewayFrame) *pb.GatewayFrame {
	serverSeq, err := l.svcCtx.Sessions.NextServerSeq(frame.GetSessionId(), frame.GetConnId(), frame.GetClientSeq())
	if err != nil {
		return buildFrame(frame, "error", 0, map[string]any{
			"type":    "error",
			"code":    session.CodeForError(err),
			"message": err.Error(),
		})
	}

	switch frame.GetFrameType() {
	case "ping":
		return buildFrame(frame, "pong", serverSeq, map[string]any{
			"type": "pong",
			"seq":  frame.GetClientSeq(),
		})
	case "resume":
		return buildFrame(frame, "session.resumed", serverSeq, map[string]any{
			"type": "session.resumed",
			"seq":  serverSeq,
			"payload": map[string]any{
				"session_id": frame.GetSessionId(),
				"gateway_id": l.svcCtx.Sessions.GatewayID(),
			},
		})
	default:
		return buildFrame(frame, "echo", serverSeq, map[string]any{
			"type": "echo",
			"seq":  serverSeq,
			"payload": map[string]any{
				"frame_type": frame.GetFrameType(),
				"message":    jsonMessage(frame.GetPayloadJson()),
			},
		})
	}
}

func jsonMessage(payload string) any {
	if payload == "" {
		return map[string]any{}
	}
	if !json.Valid([]byte(payload)) {
		return payload
	}
	return json.RawMessage(payload)
}

func buildFrame(source *pb.GatewayFrame, frameType string, serverSeq int64, payload any) *pb.GatewayFrame {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"type":"error","code":"FRAME_ENCODE_FAILED","message":"Frame encode failed"}`)
	}

	return &pb.GatewayFrame{
		SessionId:   source.GetSessionId(),
		ConnId:      source.GetConnId(),
		FrameType:   frameType,
		PayloadJson: string(data),
		ClientSeq:   source.GetClientSeq(),
		ServerSeq:   serverSeq,
	}
}
