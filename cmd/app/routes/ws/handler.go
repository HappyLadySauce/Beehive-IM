package ws

import (
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/middleware"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/router"
	wssvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/ws"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
)

// WsController handles HTTP routes for ws.
// WsController 处理ws相关 HTTP 路由。
type WsController struct {
	hub *wssvc.Hub
}

// NewWsController builds a WsController bound to the given service context.
// NewWsController 基于给定 ServiceContext 构造 WsController。
func NewWsController(svcCtx *svc.ServiceContext) *WsController {
	return &WsController{
		hub: wssvc.NewHub(nil),
	}
}

// Init validates shared handles and registers HTTP routes for the ws domain.
// Init 校验共享句柄并注册 ws 域的 HTTP 路由。
func Init(svcCtx *svc.ServiceContext) error {
	w := NewWsController(svcCtx)

	// Register routes under /ws with JWT authentication middleware.
	// 在 /ws 路径下注册路由，并使用 JWT 认证中间件。
	ws := router.V1().Group("/ws")

	ws.Use(middleware.AuthMiddleware(svcCtx))
	ws.GET("/connect", w.Connect())

	return nil
}
