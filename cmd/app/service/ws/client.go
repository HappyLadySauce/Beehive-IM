package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 << 10
	sendBufferSize = 256
)

// ClientIdentity is the identity of a client.
// ClientIdentity 是客户端的标识。
type ClientIdentity struct {
	UserID    string
	Username  string
	SessionID string
	DeviceID  string
	Platform  string
}

// Client is the client of the websocket connection.
// Client 是 WebSocket 连接的客户端。
type Client struct {
	Hub      *Hub
	Identity ClientIdentity
	Conn     *websocket.Conn

	send chan []byte
	once sync.Once
}

// NewClient creates a new client.
// NewClient 创建一个新的客户端。
func NewClient(hub *Hub, identity ClientIdentity, conn *websocket.Conn) *Client {
	return &Client{
		Hub:      hub,
		Identity: identity,
		Conn:     conn,
		send:     make(chan []byte, sendBufferSize),
	}
}

// IsSame checks if the client is the same as another client.
// IsSame 判断是否是同一个客户端。
func (c *Client) IsSame(other *Client) bool {
	if c == nil || other == nil {
		return false
	}
	return c.Identity.UserID == other.Identity.UserID &&
		c.Identity.SessionID == other.Identity.SessionID
}

// SendEnvelope queues an envelope for the write pump.
// SendEnvelope 将 envelope 放入写泵队列。
func (c *Client) SendEnvelope(envelope Envelope) error {
	if c == nil {
		return fmt.Errorf("client is nil")
	}
	if envelope.Timestamp.IsZero() {
		envelope.Timestamp = time.Now().UTC()
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("client send buffer is full")
	}
}

// ReadPump reads envelopes from the socket and dispatches them to the hub.
// ReadPump 从套接字读取 envelope 并交给 Hub 处理。
func (c *Client) ReadPump() {
	if c == nil || c.Hub == nil {
		return
	}

	defer func() {
		_ = c.Hub.Unregister(c)
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				klog.ErrorS(err, "websocket read error", "userID", c.Identity.UserID, "sessionID", c.Identity.SessionID)
			}
			return
		}

		var envelope Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			_ = c.sendProtocolError("", fmt.Sprintf("invalid envelope: %v", err))
			continue
		}

		if err := c.Hub.HandleEnvelope(context.Background(), c, envelope); err != nil {
			_ = c.sendProtocolError(envelope.ID, err.Error())
		}
	}
}

// WritePump writes queued frames to the socket.
// WritePump 将队列中的帧写入套接字。
func (c *Client) WritePump() {
	if c == nil {
		return
	}

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendProtocolError(envelopeID, message string) error {
	payload, err := json.Marshal(map[string]string{
		"message": message,
	})
	if err != nil {
		return err
	}
	return c.SendEnvelope(Envelope{
		ID:        envelopeID,
		Type:      TypeMessageError,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	})
}

// Close closes the client connection once.
// Close 关闭客户端连接（仅执行一次）。
func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.send)
		if c.Conn != nil {
			err = c.Conn.Close()
		}
	})
	return err
}
