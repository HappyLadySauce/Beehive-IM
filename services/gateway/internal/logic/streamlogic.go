package logic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
	"github.com/HappyLadySauce/Beehive-IM/services/message/messageservice"

	"github.com/zeromicro/go-zero/core/logx"
)

const messageRPCTimeout = 3 * time.Second

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

type clientEnvelope struct {
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Payload json.RawMessage `json:"payload"`
}

type messageSendPayload struct {
	ConversationID string          `json:"conversation_id"`
	ClientMsgID    string          `json:"client_msg_id"`
	ContentType    string          `json:"content_type"`
	Content        json.RawMessage `json:"content"`
	ContentJSON    json.RawMessage `json:"content_json"`
}

type messageAckPayload struct {
	ConversationID string  `json:"conversation_id"`
	AckType        string  `json:"ack_type"`
	Seqs           []int64 `json:"seqs"`
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
	case "message.send":
		return l.handleMessageSend(frame, serverSeq)
	case "message.ack":
		return l.handleMessageAck(frame, serverSeq)
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

func (l *StreamLogic) handleMessageSend(frame *pb.GatewayFrame, serverSeq int64) *pb.GatewayFrame {
	if l.svcCtx.Message == nil {
		return errorFrame(frame, serverSeq, "MESSAGE_SERVICE_UNAVAILABLE", "Message service is unavailable")
	}
	sess, err := l.svcCtx.Sessions.Get(frame.GetSessionId(), frame.GetConnId())
	if err != nil {
		return errorFrame(frame, serverSeq, session.CodeForError(err), err.Error())
	}
	var env clientEnvelope
	if err := json.Unmarshal([]byte(frame.GetPayloadJson()), &env); err != nil {
		return errorFrame(frame, serverSeq, "INVALID_FRAME", "Message frame payload is invalid")
	}
	var payload messageSendPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return errorFrame(frame, serverSeq, "INVALID_FRAME", "Message send payload is invalid")
	}
	contentJSON, err := messageContentJSON(payload)
	if err != nil {
		return errorFrame(frame, serverSeq, "INVALID_FRAME", err.Error())
	}

	rpcCtx, cancel := context.WithTimeout(l.ctx, messageRPCTimeout)
	defer cancel()
	resp, err := l.svcCtx.Message.SendMessage(rpcCtx, &messageservice.SendMessageRequest{
		ConversationId: payload.ConversationID,
		SenderId:       sess.UserID,
		DeviceId:       sess.DeviceID,
		ClientMsgId:    payload.ClientMsgID,
		ContentType:    payload.ContentType,
		ContentJson:    contentJSON,
		ClientSeq:      frame.GetClientSeq(),
	})
	if err != nil {
		l.Errorf("message send rpc failed: session_id=%s conn_id=%s error=%v", frame.GetSessionId(), frame.GetConnId(), err)
		return errorFrame(frame, serverSeq, "MESSAGE_SEND_FAILED", "Message send failed")
	}
	if !resp.GetAccepted() {
		return errorFrame(frame, serverSeq, resp.GetErrorCode(), resp.GetMessage())
	}
	return buildFrame(frame, "message.persisted", serverSeq, map[string]any{
		"type": "message.persisted",
		"seq":  serverSeq,
		"payload": map[string]any{
			"message_id":      resp.GetMessageId(),
			"conversation_id": resp.GetConversationId(),
			"message_seq":     resp.GetSeq(),
			"sender_id":       resp.GetSenderId(),
			"content_type":    resp.GetContentType(),
			"created_at":      resp.GetCreatedAt(),
			"duplicate":       resp.GetDuplicate(),
		},
	})
}

func (l *StreamLogic) handleMessageAck(frame *pb.GatewayFrame, serverSeq int64) *pb.GatewayFrame {
	if l.svcCtx.Message == nil {
		return errorFrame(frame, serverSeq, "MESSAGE_SERVICE_UNAVAILABLE", "Message service is unavailable")
	}
	sess, err := l.svcCtx.Sessions.Get(frame.GetSessionId(), frame.GetConnId())
	if err != nil {
		return errorFrame(frame, serverSeq, session.CodeForError(err), err.Error())
	}
	var env clientEnvelope
	if err := json.Unmarshal([]byte(frame.GetPayloadJson()), &env); err != nil {
		return errorFrame(frame, serverSeq, "INVALID_FRAME", "Message frame payload is invalid")
	}
	var payload messageAckPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return errorFrame(frame, serverSeq, "INVALID_FRAME", "Message ack payload is invalid")
	}

	rpcCtx, cancel := context.WithTimeout(l.ctx, messageRPCTimeout)
	defer cancel()
	resp, err := l.svcCtx.Message.AckMessages(rpcCtx, &messageservice.AckMessagesRequest{
		ConversationId: payload.ConversationID,
		UserId:         sess.UserID,
		DeviceId:       sess.DeviceID,
		AckType:        payload.AckType,
		Seqs:           payload.Seqs,
	})
	if err != nil {
		l.Errorf("message ack rpc failed: session_id=%s conn_id=%s error=%v", frame.GetSessionId(), frame.GetConnId(), err)
		return errorFrame(frame, serverSeq, "MESSAGE_ACK_FAILED", "Message ack failed")
	}
	if !resp.GetAccepted() {
		return errorFrame(frame, serverSeq, resp.GetErrorCode(), resp.GetMessage())
	}
	return buildFrame(frame, "message.ack", serverSeq, map[string]any{
		"type": "message.ack",
		"seq":  serverSeq,
		"payload": map[string]any{
			"conversation_id": payload.ConversationID,
			"ack_type":        payload.AckType,
			"updated":         resp.GetUpdated(),
		},
	})
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

func messageContentJSON(payload messageSendPayload) (string, error) {
	raw := payload.Content
	if len(payload.ContentJSON) > 0 {
		raw = payload.ContentJSON
	}
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return "", errors.New("message content is required")
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("message content_json is invalid")
		}
		value = strings.TrimSpace(value)
		if value == "" || !json.Valid([]byte(value)) {
			return "", errors.New("message content_json is invalid")
		}
		return value, nil
	}
	if !json.Valid(raw) {
		return "", errors.New("message content is invalid")
	}
	return string(raw), nil
}

func errorFrame(source *pb.GatewayFrame, serverSeq int64, code string, message string) *pb.GatewayFrame {
	if code == "" {
		code = "INTERNAL_ERROR"
	}
	if message == "" {
		message = "Request failed"
	}
	return buildFrame(source, "error", serverSeq, map[string]any{
		"type":    "error",
		"code":    code,
		"message": message,
	})
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
