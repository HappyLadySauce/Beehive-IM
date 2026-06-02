package user

import (
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/middleware"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/router"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
)

// UsersController handles HTTP routes for users.
// UsersController 处理用户相关 HTTP 路由。
type UsersController struct {
	svc *svc.ServiceContext
}

// NewUsersController builds a UsersController bound to the given service context.
// NewUsersController 基于给定 ServiceContext 构造 UsersController。
func NewUsersController(svcCtx *svc.ServiceContext) *UsersController {
	return &UsersController{
		svc: svcCtx,
	}
}

// Init validates shared handles and registers HTTP routes for the users domain.
// Init 校验共享句柄并注册 users 域的 HTTP 路由。
func Init(svcCtx *svc.ServiceContext) error {
	u := NewUsersController(svcCtx)

	// Register routes under /users with JWT authentication middleware.
	// 在 /users 路径下注册路由，并使用 JWT 认证中间件。
	users := router.V1().Group("/users")

	users.POST("/register", u.RegisterUser())

	users.Use(middleware.AuthMiddleware(svcCtx))

	return nil
}
