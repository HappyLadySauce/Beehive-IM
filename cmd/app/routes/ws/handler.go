package ws

import (
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/middleware"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/router"
	wssvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/ws"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
)

// WsController handles HTTP routes for ws.
// WsController 处理 ws 相关 HTTP 路由。
type WsController struct {
	hub *wssvc.Hub
}

// NewWsController builds a WsController bound to the given service context.
// NewWsController 基于给定 ServiceContext 构造 WsController。
func NewWsController(svcCtx *svc.ServiceContext) *WsController {
	return &WsController{
		hub: svcCtx.Hub,
	}
}

// Init registers authenticated WebSocket routes.
// Init 注册需要认证的 WebSocket 路由。
func Init(svcCtx *svc.ServiceContext) error {
	if svcCtx == nil || svcCtx.Hub == nil {
		return nil
	}
	w := NewWsController(svcCtx)
	group := router.V1().Group("/ws")
	group.Use(middleware.AuthMiddleware(svcCtx))
	group.GET("/connect", w.Connect())
	return nil
}
