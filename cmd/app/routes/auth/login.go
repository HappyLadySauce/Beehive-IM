package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	authsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

// Login authenticates a user and returns session tokens.
// Login 校验用户身份并返回会话令牌。
//
// @Summary      User login
// @Description  Authenticate with username or email and password; returns JWT access token, refresh token, and session ID. 中文：使用用户名或邮箱与密码登录，返回 JWT access token、refresh token 与 session ID。
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body v1.LoginRequest true "Login credentials"
// @Success      200 {object} v1.AuthResponse
// @Failure      400 {object} v1.ErrorResponse "Invalid request body"
// @Failure      401 {object} v1.ErrorResponse "Invalid credentials"
// @Failure      500 {object} v1.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/login [post]
func (c *AuthController) Login() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req v1.LoginRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		service := authsvc.NewAuthService(c.svc)
		resp, err := service.Login(ctx.Request.Context(), req)
		if err != nil {
			if errors.Is(err, authsvc.ErrInvalidCredentials) {
				ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
				return
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
			return
		}

		ctx.JSON(http.StatusOK, resp)
	}
}
