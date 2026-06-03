package ws

import (
	"fmt"

	"github.com/gorilla/websocket"
)

// ClientIdentity is the identity of a client.
// ClientIdentity 是客户端的标识。
type ClientIdentity struct {
	UserID string
	Username string
	SessionID string
	DeviceID string
	Platform string
}

// Client is the client of the websocket connection.
// Client 是 WebSocket 连接的客户端。
type Client struct {
	Identity ClientIdentity
	Conn	*websocket.Conn
}

// NewClient creates a new client.
// NewClient 创建一个新的客户端。
func NewClient(identity ClientIdentity, conn *websocket.Conn) *Client {
	return &Client{
		Identity: identity,
		Conn: conn,
	}
}

// IsSame checks if the client is the same as another client.
// 判断是否是同一个客户端
func (this *Client) IsSame(other *Client) bool {
	if this == nil || other == nil {
		return false
	}
	return this.Identity.UserID == other.Identity.UserID &&
		this.Identity.SessionID == other.Identity.SessionID
}

// Close closes the client.
// Close 关闭客户端。
func (c *Client) Close() error {
	if c == nil || c.Conn == nil {
		return fmt.Errorf("client or connection is nil")
	}
	return c.Conn.Close()
}
