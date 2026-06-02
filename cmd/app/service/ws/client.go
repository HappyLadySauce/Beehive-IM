package ws

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second
	maxMessageSize = 16 * 1024
	defaultSendCap = 32
)

// ClientIdentity is the authenticated identity bound to one websocket connection.
// ClientIdentity 是绑定到一条 WebSocket 连接上的认证身份。
type ClientIdentity struct {
	UserID    string
	Username  string
	SessionID string
	DeviceID  string
	Platform  string
}

// Client owns one websocket connection and its outbound queue.
// Client 持有一条 WebSocket 连接及其出站队列。
type Client struct {
	Identity ClientIdentity
	Conn     *websocket.Conn
	Send     chan Envelope
	hub      *Hub
}

func NewClient(identity ClientIdentity, conn *websocket.Conn, sendCap int) *Client {
	if sendCap <= 0 {
		sendCap = defaultSendCap
	}
	return &Client{
		Identity: identity,
		Conn:     conn,
		Send:     make(chan Envelope, sendCap),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var envelope Envelope
		if err := c.Conn.ReadJSON(&envelope); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				klog.ErrorS(err, "websocket read failed", "userID", c.Identity.UserID, "sessionID", c.Identity.SessionID)
			}
			return
		}
		if err := c.hub.HandleEnvelope(context.Background(), c.Identity, envelope); err != nil {
			c.enqueueError(envelope.ID, "invalid_message", err.Error())
			continue
		}
		c.enqueueAck(envelope.ID, envelope.Type)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteJSON(message); err != nil {
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

func (c *Client) enqueueError(messageID, code, message string) {
	envelope, err := newEnvelope(messageID, TypeError, ErrorPayload{Code: code, Message: message})
	if err != nil {
		return
	}
	select {
	case c.Send <- envelope:
	default:
		c.hub.Unregister(c)
	}
}

func (c *Client) enqueueAck(messageID, messageType string) {
	envelope, err := newEnvelope(messageID, TypeAck, AckPayload{MessageID: messageID, Type: messageType})
	if err != nil {
		return
	}
	select {
	case c.Send <- envelope:
	default:
		c.hub.Unregister(c)
	}
}
