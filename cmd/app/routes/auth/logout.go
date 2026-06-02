package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
)

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
