package conversation

import (
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/middleware"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/router"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
)

// Controller handles HTTP routes for conversations and message history.
// Controller 处理会话与历史消息 HTTP 路由。
type Controller struct {
	svc *svc.ServiceContext
}

func NewController(svcCtx *svc.ServiceContext) *Controller {
	return &Controller{svc: svcCtx}
}

// Init registers authenticated conversation routes.
// Init 注册需要认证的会话路由。
func Init(svcCtx *svc.ServiceContext) error {
	c := NewController(svcCtx)
	group := router.V1().Group("/conversations")
	group.Use(middleware.AuthMiddleware(svcCtx))
	group.GET("", c.ListConversations())
	group.GET("/:id/messages", c.ListMessages())
	return nil
}
