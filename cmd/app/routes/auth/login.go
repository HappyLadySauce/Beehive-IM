package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	authsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

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
