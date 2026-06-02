package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

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
