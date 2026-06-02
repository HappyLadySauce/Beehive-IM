package auth

import (
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/middleware"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/router"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
)

// AuthController handles HTTP routes for authentication.
// AuthController 处理认证相关 HTTP 路由。
type AuthController struct {
	svc *svc.ServiceContext
}

// NewAuthController builds an AuthController bound to the given service context.
// NewAuthController 基于给定 ServiceContext 构造 AuthController。
func NewAuthController(svcCtx *svc.ServiceContext) *AuthController {
	return &AuthController{
		svc: svcCtx,
	}
}

// Init validates shared handles and registers HTTP routes for the auth domain.
// Init 校验共享句柄并注册 auth 域的 HTTP 路由。
func Init(svcCtx *svc.ServiceContext) error {
	u := NewAuthController(svcCtx)

	// Register routes under /auth with JWT authentication middleware.
	// 在 /auth 路径下注册路由，并使用 JWT 认证中间件。
	auth := router.V1().Group("/auth")

	auth.POST("/login", u.Login())
	auth.POST("/refresh", u.Refresh())
	auth.POST("/logout", middleware.AuthMiddleware(svcCtx), u.Logout())

	return nil
}
