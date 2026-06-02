package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/user"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

// RegisterUser creates a new user account and returns session tokens.
// RegisterUser 创建新用户并返回会话令牌。
//
// @Summary      Register user
// @Description  Create a new account with username, email, and password; returns JWT access token, refresh token, and session ID. 中文：使用用户名、邮箱与密码注册新账号，返回 JWT access token、refresh token 与 session ID。
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body v1.RegisterRequest true "Registration payload"
// @Success      200 {object} v1.AuthResponse
// @Failure      400 {object} v1.ErrorResponse "Invalid request body"
// @Failure      409 {object} v1.ErrorResponse "Username or email already exists"
// @Failure      500 {object} v1.ErrorResponse "Internal server error"
// @Router       /api/v1/users/register [post]
func (c *UsersController) RegisterUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req v1.RegisterRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(400, gin.H{"error": "invalid request body"})
			return
		}

		userService := user.NewUserService(c.svc)
		resp, err := userService.Register(ctx.Request.Context(), req)
		if err != nil {
			if errors.Is(err, user.ErrUserAlreadyExists) {
				ctx.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
				return
			}

			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register user"})
			return
		}

		ctx.JSON(http.StatusOK, resp)
	}
}
