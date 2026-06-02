package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

func (a *AuthController) Login() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req v1.LoginRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		service := &auth.AuthService{
			DB:     a.svc.DB,
			Cache:  a.svc.Cache,
			Config: a.svc.Config,
		}
		resp, err := service.Login(ctx.Request.Context(), req)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
				return
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
			return
		}

		ctx.JSON(http.StatusOK, resp)
	}
}
