package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/user"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

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
