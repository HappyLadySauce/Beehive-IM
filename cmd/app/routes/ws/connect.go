package ws

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	wssvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/ws"
)

// upgrader is used to upgrade an HTTP connection to a WebSocket connection.
// upgrader 用于将 HTTP 连接升级为 WebSocket 连接。
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Connect upgrades an HTTP connection to a WebSocket connection and handles the WebSocket connection.
// Connect 将 HTTP 连接升级为 WebSocket 连接并处理 WebSocket 连接。
//
// @Summary      WebSocket connect
// @Description  Upgrade to WebSocket after JWT authentication; on success returns HTTP 101 Switching Protocols and exchanges JSON Envelope frames. 中文：JWT 认证后将 HTTP 连接升级为 WebSocket；成功时返回 HTTP 101，帧格式为 JSON Envelope。
// @Tags         ws
// @Security     BearerAuth
// @Failure      401 {object} v1.ErrorResponse "Unauthorized"
// @Failure      500 {object} v1.ErrorResponse "Failed to upgrade connection"
// @Router       /api/v1/ws/connect [get]
func (c *WsController) Connect() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade to WebSocket"})
			return
		}

		client := wssvc.NewClient(wssvc.ClientIdentity{
			UserID:    ctx.GetString("userID"),
			Username:  ctx.GetString("username"),
			SessionID: ctx.GetString("sessionID"),
			DeviceID:  ctx.GetString("deviceID"),
			Platform:  ctx.GetString("platform"),
		}, conn, 0)
		c.hub.Register(client)

		go client.WritePump()
		client.ReadPump()
	}
}
