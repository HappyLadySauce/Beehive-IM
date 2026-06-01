package user

import (
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
		token, err := userService.Register(req)
		if err != nil {
			ctx.JSON(500, gin.H{"error": "failed to register user"})
			return
		}

		ctx.JSON(200, gin.H{"token": token})
	}
}