package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
)

// Logout invalidates the current session.
// Logout 使当前会话失效。
//
// @Summary      User logout
// @Description  Delete the authenticated session; requires a valid Bearer access token. 中文：注销当前会话；需要有效的 Bearer access token。
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} v1.MessageResponse
// @Failure      401 {object} v1.ErrorResponse "Missing or invalid session"
// @Failure      500 {object} v1.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/logout [post]
func (c *AuthController) Logout() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		sessionID := ctx.GetString("sessionID")
		if sessionID == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "missing session"})
			return
		}

		service := authsvc.NewAuthService(c.svc)
		if err := service.DeleteSession(ctx.Request.Context(), sessionID); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}
