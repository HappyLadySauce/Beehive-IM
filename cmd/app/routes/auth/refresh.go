package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

// Refresh exchanges a valid refresh token for a new access token.
// Refresh 使用有效的 refresh token 换取新的 access token。
//
// @Summary      Refresh access token
// @Description  Rotate session credentials using session_id and refresh_token. 中文：通过 session_id 与 refresh_token 刷新会话凭证。
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body v1.RefreshRequest true "Refresh credentials"
// @Success      200 {object} v1.AuthResponse
// @Failure      400 {object} v1.ErrorResponse "Invalid request body"
// @Failure      401 {object} v1.ErrorResponse "Invalid refresh token"
// @Router       /api/v1/auth/refresh [post]
func (c *AuthController) Refresh() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req v1.RefreshRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		service := authsvc.NewAuthService(c.svc)
		resp, err := service.RefreshSessionToken(ctx.Request.Context(), req)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}

		ctx.JSON(http.StatusOK, resp)
	}
}
