package ws

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	wssvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/ws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Connect upgrades HTTP to WebSocket and runs the client pumps.
// Connect 将 HTTP 升级为 WebSocket 并运行客户端读写泵。
func (c *WsController) Connect() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if c.hub == nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "websocket hub is not configured"})
			return
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade to WebSocket"})
			return
		}

		client := wssvc.NewClient(c.hub, wssvc.ClientIdentity{
			UserID:    ctx.GetString("userID"),
			Username:  ctx.GetString("username"),
			SessionID: ctx.GetString("sessionID"),
			DeviceID:  ctx.GetString("deviceID"),
			Platform:  ctx.GetString("platform"),
		}, conn)

		if err := c.hub.Register(client); err != nil {
			_ = client.Close()
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register websocket client"})
			return
		}

		go client.WritePump()
		client.ReadPump()
	}
}
